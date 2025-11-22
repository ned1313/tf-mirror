# Job Processing Test Instructions

## Prerequisites

1. **Start MinIO** (in a terminal):
   ```powershell
   docker-compose -f docker-compose.dev.yml up -d
   ```

2. **Verify MinIO is running**:
   - Open http://localhost:9001 in your browser
   - Login with: minioadmin / minioadmin
   - Verify the `terraform-mirror` bucket exists

## Running the Test

### Option 1: Automated Test Script

Run the PowerShell test script:
```powershell
.\test-job-processing.ps1
```

This will:
1. Start the server in a new PowerShell window
2. Upload the test-providers.hcl file
3. Display the job status and items
4. List all jobs
5. Show MinIO console URL for verification

### Option 2: Manual Testing

**Terminal 1 - Start Server:**
```powershell
$env:AWS_ACCESS_KEY_ID = "minioadmin"
$env:AWS_SECRET_ACCESS_KEY = "minioadmin"
.\terraform-mirror.exe --config config.dev.hcl
```

**Terminal 2 - Create Job:**
```powershell
# Upload HCL file to create a job
curl -X POST -F "file=@test-providers.hcl" http://localhost:8080/admin/api/providers/load

# Expected response:
# {"job_id":1,"message":"Provider loading job created and completed: 2 total providers","total_providers":2}
```

**Terminal 2 - Check Job Status:**
```powershell
# Get job details (replace 1 with your job ID)
curl http://localhost:8080/admin/api/jobs/1 | ConvertFrom-Json | ConvertTo-Json -Depth 10

# List all jobs
curl http://localhost:8080/admin/api/jobs | ConvertFrom-Json | ConvertTo-Json
```

## What to Verify

### 1. Job Creation
- ✅ Job ID is returned (positive integer)
- ✅ Message indicates job was created
- ✅ Total providers count matches HCL file (2 providers)

### 2. Job Status
- ✅ Status is "completed" (or "failed" if registry is unreachable)
- ✅ Progress is 100%
- ✅ Total items is 5 (random: 2 versions × 2 platforms + null: 1 version × 1 platform)
- ✅ Completed items matches successful downloads
- ✅ Failed items shows any failures

### 3. Job Items
Each item should show:
- ✅ Namespace (hashicorp)
- ✅ Type (random or null)
- ✅ Version (3.6.0, 3.6.3, or 3.2.2)
- ✅ Platform (linux_amd64 or windows_amd64)
- ✅ Status (completed or failed)
- ✅ Error message (if failed)

### 4. MinIO Storage
1. Open http://localhost:9001
2. Login: minioadmin / minioadmin
3. Navigate to `terraform-mirror` bucket
4. Verify provider files were uploaded:
   - Path format: `providers/{namespace}/{type}/{version}/terraform-provider-{type}_{version}_{platform}.zip`
   - Example: `providers/hashicorp/random/3.6.0/terraform-provider-random_3.6.0_linux_amd64.zip`

### 5. Database
Check the database file `terraform-mirror-dev.db`:
- ✅ Job record exists in `download_jobs` table
- ✅ Job items exist in `download_job_items` table
- ✅ Provider records exist in `providers` table

## Expected Behavior

**Current (Step 10 - Synchronous):**
- Upload request blocks until all providers are downloaded
- Request takes 2-10 seconds depending on network speed
- Returns job ID when complete
- Job status is "completed" or "failed" immediately

**Future (Step 12 - Asynchronous):**
- Upload request returns immediately with job ID
- Processing happens in background
- Job status transitions: pending → running → completed/failed
- Can upload multiple files concurrently
- Poll job status endpoint to track progress

## Troubleshooting

**Server won't start:**
- Check if port 8080 is already in use
- Verify MinIO is running on ports 9000/9001
- Check environment variables are set

**Job fails:**
- Verify internet connection (needs to download from releases.hashicorp.com)
- Check MinIO bucket exists and is accessible
- Look at error_message in job items for details

**No files in MinIO:**
- Verify AWS credentials in environment
- Check server logs for S3 upload errors
- Ensure MinIO bucket is created and accessible

## Cleanup

```powershell
# Stop MinIO
docker-compose -f docker-compose.dev.yml down

# Remove database (if you want to start fresh)
Remove-Item terraform-mirror-dev.db

# Remove cache
Remove-Item -Recurse -Force ./cache
```
