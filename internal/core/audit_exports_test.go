package core_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// runExports runs the export worker and fetches the export, which must be
// ready.
func runExport(t *testing.T, e *testenv.Env, token, id string) map[string]any {
	t.Helper()
	if _, err := e.Core.RunAuditExports(context.Background()); err != nil {
		t.Fatalf("worker: %v", err)
	}
	x := e.Get("/audit-exports/"+id, token).Expect(200).JSON()
	if x["status"] != "ready" || x["download_url"] == nil {
		t.Fatalf("export = %v", x)
	}
	return x
}

func download(t *testing.T, e *testenv.Env, x map[string]any) []byte {
	t.Helper()
	b, ok := e.Exports.Download(x["download_url"].(string))
	if !ok {
		t.Fatalf("no file behind %v", x["download_url"])
	}
	return b
}

// API Reference → Audit → Exporting.
func TestAuditExport(t *testing.T) {
	e := testenv.New(t)
	u := e.Signup()
	app := e.App(system, nil)
	e.Grant(e.Admin, u.ID, app)
	e.Patch("/users/"+u.ID+"/profile", e.Admin, map[string]any{"display_name": "Renamed"}).Expect(200)
	want := e.Get("/audit-events?target_user_id="+u.ID, e.Admin).Expect(200).JSON()["data"].([]any)
	if len(want) < 3 {
		t.Fatalf("only %d events to export", len(want))
	}

	// NDJSON, by audit.view: every matching event, oldest first.
	req := e.PostIdem("/audit-exports", e.Admin, testenv.Unique(), map[string]any{"target_user_id": u.ID}).Expect(202).JSON()
	if req["status"] != "pending" || req["download_url"] != nil || req["format"] != "ndjson" {
		t.Fatalf("request = %v", req)
	}
	id := req["id"].(string)
	x := runExport(t, e, e.Admin, id)
	var got []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(download(t, e, x)))
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("line %q: %v", sc.Text(), err)
		}
		got = append(got, ev)
	}
	if len(got) != len(want) || int(x["row_count"].(float64)) != len(want) {
		t.Fatalf("exported %d rows (row_count %v), want %d", len(got), x["row_count"], len(want))
	}
	if got[0]["id"] != want[len(want)-1].(map[string]any)["id"] {
		t.Fatal("export should be oldest first")
	}

	// The export is itself audited, as its requester.
	exported := e.Get("/audit-events?action=audit.exported&target_id="+id, e.Admin).Expect(200).JSON()["data"].([]any)
	if len(exported) != 1 || exported[0].(map[string]any)["actor"].(map[string]any)["id"] != e.AdminID {
		t.Fatalf("audit.exported = %v", exported)
	}

	// Idempotent: the same key returns the same export.
	key := testenv.Unique()
	a := e.PostIdem("/audit-exports", e.Admin, key, map[string]any{"format": "csv", "target_user_id": u.ID}).Expect(202).Str("id")
	b := e.PostIdem("/audit-exports", e.Admin, key, map[string]any{"format": "csv", "target_user_id": u.ID}).Expect(202).Str("id")
	if a != b {
		t.Fatalf("idempotent request made two exports: %s, %s", a, b)
	}
	// CSV: a header, then one row per event.
	rows, err := csv.NewReader(bytes.NewReader(download(t, e, runExport(t, e, e.Admin, a)))).ReadAll()
	if err != nil || len(rows) != len(want)+1 || rows[0][0] != "id" || rows[0][2] != "action" {
		t.Fatalf("csv: %d rows, %v", len(rows), err)
	}

	// Validation.
	e.PostIdem("/audit-exports", e.Admin, testenv.Unique(), map[string]any{"format": "xml"}).ExpectErr(422, "validation_failed")

	// A User exports only their own trail, and sees only their own exports.
	e.PostIdem("/audit-exports", u.Token, testenv.Unique(), map[string]any{}).ExpectErr(403, "forbidden")
	own := e.PostIdem("/audit-exports", u.Token, testenv.Unique(), map[string]any{"target_user_id": "me"}).Expect(202).Str("id")
	if n := int(runExport(t, e, u.Token, own)["row_count"].(float64)); n < len(want) {
		t.Fatalf("own trail exported %d rows, want at least %d", n, len(want))
	}
	e.Get("/audit-exports/"+id, u.Token).ExpectErr(404, "audit_export_not_found")

	// An app-confined key is forced to its own Application.
	appKey := e.Post("/api-keys", e.Admin, map[string]any{"name": "auditor", "scope": app,
		"permissions": []string{"audit.view"}}).Expect(201).Str("secret")
	scoped := e.PostIdem("/audit-exports", appKey, testenv.Unique(), map[string]any{"target_user_id": u.ID}).Expect(202).Str("id")
	sc = bufio.NewScanner(bytes.NewReader(download(t, e, runExport(t, e, appKey, scoped))))
	n := 0
	for sc.Scan() {
		var ev map[string]any
		_ = json.Unmarshal(sc.Bytes(), &ev)
		if ev["application_id"] != app {
			t.Fatalf("app key exported another Application's event: %v", ev)
		}
		n++
	}
	if n == 0 {
		t.Fatal("app key exported nothing")
	}
}
