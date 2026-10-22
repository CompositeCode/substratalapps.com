package core

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/api/gen"
	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
)

// ExportStore holds finished Audit Event exports: a private S3 bucket in
// AWS (objects deleted after 7 days by a lifecycle rule), a local
// directory otherwise.
type ExportStore interface {
	Put(ctx context.Context, key, contentType string, body io.ReadSeeker) error
	// URL returns a link that downloads key for ttl.
	URL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

const (
	auditExportTTL      = 7 * 24 * time.Hour
	auditExportURLTTL   = 15 * time.Minute
	auditExportMaxRows  = 1_000_000
	auditExportPageSize = 1000
	// A running export whose worker died is picked up again after this.
	auditExportStale = 20 * time.Minute
)

type auditExportRow struct {
	ID              string          `json:"id"`
	RequestedByType string          `json:"requested_by_type"`
	RequestedByID   string          `json:"requested_by_id"`
	Scope           auditForced     `json:"scope"`
	Filters         json.RawMessage `json:"filters"`
	Format          string          `json:"format"`
	Status          string          `json:"status"`
	RowCount        *int            `json:"row_count"`
	ObjectKey       *string         `json:"object_key"`
	Error           *string         `json:"error"`
	TestMode        bool            `json:"test_mode"`
	CreatedAt       time.Time       `json:"created_at"`
	CompletedAt     *time.Time      `json:"completed_at"`
	ExpiresAt       time.Time       `json:"expires_at"`
}

// auditExport is the API AuditExport object.
type auditExport struct {
	ID                   string          `json:"id"`
	Status               string          `json:"status"`
	Format               string          `json:"format"`
	Filters              json.RawMessage `json:"filters"`
	RowCount             *int            `json:"row_count"`
	DownloadURL          *string         `json:"download_url"`
	DownloadURLExpiresAt *time.Time      `json:"download_url_expires_at"`
	Error                *string         `json:"error"`
	CreatedAt            time.Time       `json:"created_at"`
	CompletedAt          *time.Time      `json:"completed_at"`
	ExpiresAt            time.Time       `json:"expires_at"`
	TestMode             bool            `json:"test_mode"`
}

// exportObject builds the API object, signing a fresh download link when
// the file is ready.
func (c *Call) exportObject(x auditExportRow) (auditExport, error) {
	out := auditExport{ID: x.ID, Status: x.Status, Format: x.Format, Filters: x.Filters, RowCount: x.RowCount,
		Error: x.Error, CreatedAt: x.CreatedAt, CompletedAt: x.CompletedAt, ExpiresAt: x.ExpiresAt, TestMode: x.TestMode}
	if x.Status == "ready" && x.ObjectKey != nil {
		u, err := c.s.Exports.URL(c.ctx, *x.ObjectKey, auditExportURLTTL)
		if err != nil {
			return out, err
		}
		exp := c.now.Add(auditExportURLTTL)
		out.DownloadURL, out.DownloadURLExpiresAt = &u, &exp
	}
	return out, nil
}

func exportsUnavailable() error {
	return httpx.E(503, "service_unavailable", "Audit exports aren't configured in this environment.")
}

// ── POST /audit-exports ────────────────────────────────────────────────

func (s *Server) AuditCreateExport(w http.ResponseWriter, r *http.Request, params gen.AuditCreateExportParams) {
	s.serve(w, r, opts{op: "audit.createExport", idem: idemRequired}, func(c *Call) error {
		if c.s.Exports == nil {
			return exportsUnavailable()
		}
		var in struct {
			Format *string `json:"format"`
			auditFilters
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		var f httpx.Fields
		format := "ndjson"
		if in.Format != nil {
			format = *in.Format
			if format != "ndjson" && format != "csv" {
				f.Enum("format", "ndjson", "csv")
			}
		}
		if in.ActorType != nil {
			switch *in.ActorType {
			case "user", "api_key", "system":
			default:
				f.Enum("actor_type", "user", "api_key", "system")
			}
		}
		if len(in.Action) > 100 {
			f.Max("action", "too_long", 100)
		}
		if in.Since != nil && in.Until != nil && !in.Since.Before(*in.Until) {
			f.Add("until", "out_of_range").Message = "until must be after since"
		}
		if err := f.Err(); err != nil {
			return err
		}
		filters := in.auditFilters
		if filters.TargetUserID != nil {
			uid, err := c.selfID(*filters.TargetUserID)
			if err != nil {
				return err
			}
			filters.TargetUserID = &uid
		}
		forced, err := c.auditScope(filters.TargetUserID, filters.TenantID)
		if err != nil {
			return err
		}
		typ, by := c.actor()
		if err := c.hit("auditexport:"+by, perDay(20), false); err != nil {
			return err
		}
		var x auditExportRow
		if err := db.One(c.ctx, c.q, &x, `insert into audit_exports (id, requested_by_type, requested_by_id, scope, filters,
				format, test_mode, expires_at)
			values (:id, :bt, :bi, :scope::jsonb, :filters::jsonb, :fmt, :tm, :exp::timestamptz)
			returning to_jsonb(audit_exports)`,
			db.Args{"id": ids.New(ids.AuditExport), "bt": typ, "bi": by, "scope": db.JSON(forced), "filters": db.JSON(filters),
				"fmt": format, "tm": c.testMode(), "exp": db.Time(c.now.Add(auditExportTTL))}); err != nil {
			return err
		}
		out, err := c.exportObject(x)
		if err != nil {
			return err
		}
		return c.Accepted(out)
	})
}

// ── GET /audit-exports/{id} ────────────────────────────────────────────

func (s *Server) AuditGetExport(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "audit.getExport"}, func(c *Call) error {
		if c.s.Exports == nil {
			return exportsUnavailable()
		}
		var x auditExportRow
		err := db.One(c.ctx, c.q, &x, `select to_jsonb(x) from audit_exports x where id = :id`, db.Args{"id": id})
		typ, by := c.actor()
		if err == db.ErrNotFound || (err == nil && !(x.RequestedByType == typ && x.RequestedByID == by) && !c.can("audit.view")) {
			return httpx.NotFoundResource("audit_export")
		}
		if err != nil {
			return err
		}
		out, err := c.exportObject(x)
		if err != nil {
			return err
		}
		return c.OK(out)
	})
}

// ── Worker (jobs: audit-exports, every minute) ─────────────────────────

// RunAuditExports writes every due export, one at a time, in both modes.
func (s *Server) RunAuditExports(ctx context.Context) (done int, err error) {
	if s.Exports == nil {
		return 0, nil
	}
	err = eachMode(func(tm bool) error {
		for {
			var claimed []auditExportRow
			if err := s.system(ctx, tm, db.Settings{}, func(c *Call) error {
				var err error
				claimed, err = db.All[auditExportRow](ctx, c.q, `update audit_exports set status = 'running', started_at = now()
					where id = (select id from audit_exports
						where status = 'pending' or (status = 'running' and started_at < now() - make_interval(secs => :stale))
						order by created_at limit 1 for update skip locked)
					returning to_jsonb(audit_exports)`, db.Args{"stale": auditExportStale.Seconds()})
				return err
			}); err != nil {
				return err
			}
			if len(claimed) == 0 {
				return nil
			}
			if err := s.runAuditExport(ctx, tm, claimed[0]); err != nil {
				return err
			}
			done++
		}
	})
	return
}

// runAuditExport writes one export's file and marks it ready, or failed.
func (s *Server) runAuditExport(ctx context.Context, tm bool, x auditExportRow) error {
	var filters auditFilters
	if err := json.Unmarshal(x.Filters, &filters); err != nil {
		return s.failAuditExport(ctx, tm, x, "internal_error")
	}
	file, err := os.CreateTemp("", "audit-export-*")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()

	var write func(auditEvent) error
	var flush func() error
	contentType, ext := "application/x-ndjson", "ndjson"
	if x.Format == "csv" {
		contentType, ext = "text/csv", "csv"
		cw := csv.NewWriter(file)
		if err := cw.Write(auditCSVHeader); err != nil {
			return err
		}
		write = func(e auditEvent) error { return cw.Write(auditCSVRow(e)) }
		flush = func() error { cw.Flush(); return cw.Error() }
	} else {
		enc := json.NewEncoder(file)
		write = func(e auditEvent) error { return enc.Encode(e) }
		flush = func() error { return nil }
	}

	// Page through in (timestamp, id) order, one short transaction per
	// page, so a large export never holds everything in memory.
	count := 0
	var after *auditEvent
	for {
		var page []auditEvent
		if err := s.system(ctx, tm, db.Settings{}, func(c *Call) error {
			where, args := auditWhere(x.Scope, filters)
			if after != nil {
				where = append(where, "(a.timestamp, a.id) > (:ps::timestamptz, :pi)")
				args["ps"], args["pi"] = db.Time(after.Timestamp), after.ID
			}
			if len(where) == 0 {
				where = []string{"true"}
			}
			args["lim"] = int64(auditExportPageSize)
			var err error
			page, err = db.All[auditEvent](ctx, c.q, auditSQL+" where "+strings.Join(where, " and ")+
				" order by a.timestamp, a.id limit :lim", args)
			return err
		}); err != nil {
			return err
		}
		for _, e := range page {
			if err := write(e); err != nil {
				return err
			}
		}
		count += len(page)
		if count > auditExportMaxRows {
			return s.failAuditExport(ctx, tm, x, "too_many_rows")
		}
		if len(page) < auditExportPageSize {
			break
		}
		after = &page[len(page)-1]
	}
	if err := flush(); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	prefix := "live"
	if tm {
		prefix = "test"
	}
	key := "exports/" + prefix + "/" + x.ID + "." + ext
	if err := s.Exports.Put(ctx, key, contentType, file); err != nil {
		return err
	}
	return s.system(ctx, tm, db.Settings{}, func(c *Call) error {
		if _, err := c.q.Exec(ctx, `update audit_exports set status = 'ready', row_count = :n, object_key = :k,
			completed_at = now() where id = :id`, db.Args{"id": x.ID, "n": int64(count), "k": key}); err != nil {
			return err
		}
		return c.audit(auditExportedEvent(x, count))
	})
}

func (s *Server) failAuditExport(ctx context.Context, tm bool, x auditExportRow, reason string) error {
	return s.system(ctx, tm, db.Settings{}, func(c *Call) error {
		_, err := c.q.Exec(ctx, `update audit_exports set status = 'failed', error = :e, completed_at = now() where id = :id`,
			db.Args{"id": x.ID, "e": reason})
		return err
	})
}

// auditExportedEvent records who exported what, in the export's own scope,
// so it shows up wherever the exported events would.
func auditExportedEvent(x auditExportRow, count int) Audit {
	a := Audit{Action: "audit.exported", TargetType: "audit_export", TargetID: x.ID,
		After:     map[string]any{"filters": x.Filters, "format": x.Format, "row_count": count},
		ActorType: x.RequestedByType, ActorID: x.RequestedByID}
	if v := x.Scope.ApplicationID; v != "" {
		a.ApplicationID = &v
	}
	if v := x.Scope.TargetUserID; v != "" {
		a.TargetUserID = &v
	}
	if v := x.Scope.TenantID; v != "" {
		a.TenantID = &v
	}
	return a
}

var auditCSVHeader = []string{"id", "timestamp", "action", "actor_type", "actor_id", "target_type", "target_id",
	"target_user_id", "application_id", "organization_id", "tenant_id", "request_id", "before", "after"}

func auditCSVRow(e auditEvent) []string {
	return []string{e.ID, e.Timestamp.UTC().Format(time.RFC3339Nano), e.Action, e.Actor.Type, e.Actor.ID,
		e.Target.Type, e.Target.ID, deref(e.TargetUserID), deref(e.ApplicationID), deref(e.OrganizationID),
		deref(e.TenantID), deref(e.RequestID), jsonCell(e.Before), jsonCell(e.After)}
}

func jsonCell(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	return string(raw)
}
