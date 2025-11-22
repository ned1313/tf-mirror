package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGetJob_NotFound(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/admin/api/jobs/999", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var response map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "job_not_found", response["error"])
}

func TestHandleGetJob_InvalidID(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/admin/api/jobs/invalid", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "invalid_job_id", response["error"])
}

func TestHandleListJobs_Empty(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/admin/api/jobs", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response jobListResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 0, len(response.Jobs))
	assert.Equal(t, 0, response.Total)
	assert.Equal(t, 10, response.Limit)
	assert.Equal(t, 0, response.Offset)
}

func TestHandleListJobs_WithCustomPagination(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/admin/api/jobs?limit=5&offset=10", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response jobListResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 5, response.Limit)
	assert.Equal(t, 10, response.Offset)
}

func TestJobIntegration_CreateAndRetrieve(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create a provider loading job by uploading HCL
	hcl := `
provider "hashicorp/random" {
  versions  = ["3.6.0"]
  platforms = ["linux_amd64"]
}
`

	req, _ := createMultipartRequest(t, hcl)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "Failed to create job: %s", rr.Body.String())

	var createResponse LoadProvidersResponse
	err := json.NewDecoder(rr.Body).Decode(&createResponse)
	require.NoError(t, err)
	require.Greater(t, createResponse.JobID, int64(0), "Job ID should be positive")

	// Now retrieve the job using proper ID formatting
	jobURL := "/admin/api/jobs/" + strconv.FormatInt(createResponse.JobID, 10)
	req = httptest.NewRequest("GET", jobURL, nil)
	rr = httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "Failed to get job: %s", rr.Body.String())

	var jobResp jobResponse
	err = json.NewDecoder(rr.Body).Decode(&jobResp)
	require.NoError(t, err)

	assert.Equal(t, createResponse.JobID, jobResp.ID)
	assert.Equal(t, "hcl", jobResp.SourceType)
	assert.Equal(t, "completed", jobResp.Status)
	assert.Equal(t, 100, jobResp.Progress)
	assert.Greater(t, jobResp.TotalItems, 0)
	assert.NotNil(t, jobResp.Items)
	assert.Greater(t, len(jobResp.Items), 0)

	// Verify job appears in list
	req = httptest.NewRequest("GET", "/admin/api/jobs", nil)
	rr = httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var listResp jobListResponse
	err = json.NewDecoder(rr.Body).Decode(&listResp)
	require.NoError(t, err)

	assert.Equal(t, 1, len(listResp.Jobs))
	assert.Equal(t, createResponse.JobID, listResp.Jobs[0].ID)
}
