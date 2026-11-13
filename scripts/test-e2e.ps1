# Run E2E Tests across all services
param(
    [string]$Target = "all"
)

$ErrorActionPreference = "Continue"
$failed = 0

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host " Running E2E Tests (Target: $Target)" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# TypeScript Services with e2e
$tsServices = @(
    "services/api-gateway",
    "services/delivery-service",
    "services/notification-service",
    "services/realtime-service",
    "services/user-service"
)

# Go Services with e2e
$goServices = @(
    "services/analytics-service",
    "services/driver-service",
    "services/media-service",
    "services/payment-service",
    "services/search-service"
)

if ($Target -ne "all") {
    $tsServices = $tsServices | Where-Object { $_ -match $Target }
    $goServices = $goServices | Where-Object { $_ -match $Target }
}

foreach ($srv in $tsServices) {
    if (Test-Path "$srv/test/jest-e2e.json") {
        Write-Host "`n---> Running E2E Tests for $srv (Node/Jest)..." -ForegroundColor Yellow
        npm --prefix $srv run test:e2e -- --passWithNoTests
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[FAIL] E2E tests failed in $srv" -ForegroundColor Red
            $failed++
        } else {
            Write-Host "[PASS] E2E tests passed in $srv" -ForegroundColor Green
        }
    }
}

foreach ($srv in $goServices) {
    if ((Test-Path "$srv/go.mod") -and (Test-Path "$srv/test/e2e")) {
        Write-Host "`n---> Running E2E Tests for $srv (Go)..." -ForegroundColor Yellow
        Push-Location $srv
        try {
            go test -v ./test/e2e/...
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[FAIL] E2E tests failed in $srv" -ForegroundColor Red
                $failed++
            } else {
                Write-Host "[PASS] E2E tests passed in $srv" -ForegroundColor Green
            }
        } finally {
            Pop-Location
        }
    }
}

Write-Host "`n==========================================" -ForegroundColor Cyan
if ($failed -gt 0) {
    Write-Host " E2E tests finished with $failed failure(s)" -ForegroundColor Red
    exit 1
} else {
    Write-Host " All E2E tests passed successfully!" -ForegroundColor Green
    exit 0
}

