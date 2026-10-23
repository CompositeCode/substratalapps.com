---
layout: default
title: Interactive Reference
parent: OpenAPI Specification
nav_order: 1
permalink: /openapi/reference/
---

# Interactive Reference
{: .no_toc }

Every operation, parameter, request body, and response in [`openapi.yaml`]({{ site.baseurl }}/openapi.yaml), rendered with Swagger UI.
{: .fs-6 .fw-300 }

**To try a request against the local devserver:**

1. Start the devserver with this page's origin allowed. For example, `CORS_ALLOWED_ORIGINS={{ site.url }} make run`, or `http://localhost:4000` if you're running the docs site locally.
2. Choose **Local devserver** under **Servers**.
3. Click **Authorize** and paste a user access token. API Keys are refused from a browser (`400 api_key_in_browser`), so use `curl` or Postman for those.
4. Open an operation and click **Try it out**.

Browser requests to production are blocked by CORS on purpose. The production API allows only first-party browser origins, so admin and service credentials never get used from a web page (see [Non-Functional Requirements → CORS](../../non-functional-requirements/#cors)). To call production, use `curl`, a generated client, or an API tool such as Postman. Every operation's request is shown here for you to copy.

[Download openapi.yaml]({{ site.baseurl }}/openapi.yaml){: .btn .btn-outline } [How to read the spec](../){: .btn .btn-outline }

<div id="swagger-ui" class="swagger-host">
  <noscript>The interactive reference needs JavaScript. The raw document is at <a href="{{ site.baseurl }}/openapi.yaml">openapi.yaml</a>.</noscript>
</div>

<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.33.3/swagger-ui.css">
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.33.3/swagger-ui-bundle.js"></script>
<script src="https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.min.js"></script>
<script>
  (function () {
    var specUrl = "{{ site.baseurl }}/openapi.yaml";
    var host = document.getElementById("swagger-ui");

    function render(spec) {
      // Offer the local devserver next to production, without changing the published document.
      spec.servers = (spec.servers || []).concat([
        { url: "http://localhost:8080", description: "Local devserver (make run)" }
      ]);
      SwaggerUIBundle({
        spec: spec,
        domNode: host,
        deepLinking: true,
        docExpansion: "none",
        defaultModelsExpandDepth: 0,
        displayOperationId: true,
        displayRequestDuration: true,
        filter: true,
        persistAuthorization: true,
        tryItOutEnabled: false,
        // Every API request needs Substratal-Version (Conventions → Versioning);
        // the document doesn't declare it per operation.
        requestInterceptor: function (req) {
          req.headers["Substratal-Version"] = "1";
          return req;
        }
      });
    }

    fetch(specUrl)
      .then(function (r) { if (!r.ok) throw new Error(r.status); return r.text(); })
      .then(function (text) { render(jsyaml.load(text)); })
      .catch(function (err) {
        host.innerHTML = '<p>Couldn’t load the specification (' + err.message +
          '). The raw document is at <a href="' + specUrl + '">openapi.yaml</a>.</p>';
      });
  })();
</script>
