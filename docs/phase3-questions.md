# Phase 3: Module Registry Mirror - Implementation Questions

This document captures questions that need to be answered before implementing Phase 3.

---

## 1. Module Source Address Handling

The planning doc mentions that users will update their module source to include the mirror hostname (e.g., `mirror.hostname.local/terraform-aws-modules/iam/aws`).

**Q1.1:** Should the mirror automatically rewrite any nested module references within downloaded modules, or will users be responsible for ensuring nested modules also reference the mirror?

**Answer:** Nested modules that use a remote location will need to be updated dynamically by the mirror. Nested modules that use a local file path should be fine.

**Q1.2:** Should there be a configuration option to set a "source registry" hostname (defaulting to `registry.terraform.io`) for upstream fetches?

**Answer:** Yes. We'll assume registry.terraform.io unless otherwise specified.

---

## 2. Module Storage Structure

Providers have a clear structure: `namespace/type/version/platform`. Modules have `namespace/name/system/version`.

**Q2.1:** What S3 key structure do you prefer for modules? For example:

- `modules/{namespace}/{name}/{system}/{version}/{filename}` (parallel to providers)
- Something else?

**Answer:** The suggest key structure is fine.

**Q2.2:** Modules are typically distributed as tarballs (`.tar.gz`) from GitHub. Should we store the original tarball or extract/repackage it?

**Answer:** We'll need to extract and repackage due to the need to update nested modules with a remote source to the new source location.

---

## 3. Database Schema for Modules

**Q3.1:** Should modules have similar status flags as providers (`deprecated`, `blocked`)?

**Answer:** Yes.

**Q3.2:** Should we track the original source URL (e.g., GitHub repo) for audit/debugging purposes?

**Answer:** Yes.

**Q3.3:** Any additional metadata fields you want to capture (e.g., description, documentation URL, submodules list)?

**Answer:** No.

---

## 4. Auto-Download Behavior

The planning doc mentions auto-download can be disabled by administrators.

**Q4.1:** Should auto-download for modules be a separate configuration flag from providers (`auto_download_modules` vs `auto_download_providers`)?

**Answer:** Yes

**Q4.2:** Should there be size limits specific to modules (separate from `max_download_size_mb`)?

**Answer:** No

---

## 5. HCL Definition File Format for Modules

Providers use an HCL file to pre-load. Modules will need similar functionality.

**Q5.1:** What format do you prefer for the module definition file? For example:

```hcl
module {
  source  = "terraform-aws-modules/vpc/aws"
  versions = ["5.0.0", "5.1.0", "~> 4.0"]  # Support constraints?
}
```

**Answer:** Module versions should be exact and not ranges. Similar to the way providers are handled.

**Q5.2:** Should version constraints (like `~> 4.0`) be supported, downloading all matching versions from the upstream registry?

**Answer:** No. Specific versions only.

---

## 6. Version Discovery from Upstream

Unlike providers (where we have explicit checksum files), modules don't have a verification mechanism.

**Q6.1:** Should we simply trust the upstream registry and GitHub, or implement any verification (e.g., checksum of downloaded tarball)?

**Answer:** Trust the upstream.

**Q6.2:** Should we cache the version list from upstream, or always fetch fresh when auto-download is enabled?

**Answer:** Fetch fresh

---

## 7. Admin UI Additions

**Q7.1:** Should modules have their own dedicated view (like Providers), or be combined into a "Registry" view with tabs?

**Answer:** Combined registry view

**Q7.2:** Should the dashboard show module statistics alongside provider statistics?

**Answer:** Yes

---

## 8. Job System Integration

**Q8.1:** Should module downloads use the same job system as providers (with `job_type = 'module'`), or have a separate table/system?

**Answer:** Same job system.

**Q8.2:** For the job item table, should we add module-specific fields, or create a separate `module_job_items` table?

**Answer:** Probably a separate table. Modules may have some field distinct from providers.

---

## 9. API Endpoint Structure

Based on the protocol, we need:

- `/.well-known/terraform.json` → add `"modules.v1": "/v1/modules/"`
- `GET /v1/modules/{namespace}/{name}/{system}/versions`
- `GET /v1/modules/{namespace}/{name}/{system}/{version}/download`

**Q9.1:** Should admin API endpoints follow the same pattern as providers?

- `GET /admin/api/modules` - List modules
- `POST /admin/api/modules/load` - Load from HCL file
- `DELETE /admin/api/modules/{id}` - Delete module
- etc.

**Answer:** Yes

---

## 10. Phase 2 Dependencies

I notice Phase 2 (auto-download providers) is listed in the ROADMAP but marked as future work.

**Q10.1:** Should Phase 3 (modules) include auto-download for modules from the start, or should we implement manual loading first (parallel to Phase 1 for providers) and add auto-download later?

**Answer:** Auto-download has been completed for providers. It's just not marked as completed in the doc.

**Q10.2:** Is Phase 2 (provider auto-download) a prerequisite, or can we proceed with Phase 3 independently?

**Answer:** Phase 2 is complete. Check the code to confirm.

---

## Additional Notes

*Add any additional context or requirements here.*
