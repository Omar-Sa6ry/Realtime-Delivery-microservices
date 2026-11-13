# Run Test Coverage across all services and packages
param(
    [string]$Target = "all"
)

$ErrorActionPreference = "Continue"
$failed = 0

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host " Running Test Coverage (Target: $Target)" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# TypeScript Packages & Services
$tsProjects = @(
    "packages/ts",
    "services/api-gateway",
    "services/delivery-service",
    "services/notification-service",
    "services/realtime-service",
    "services/user-service"
)

# Go Packages & Services
$goProjects = @(
    "packages/go",
    "services/analytics-service",
    "services/driver-service",
    "services/media-service",
    "services/payment-service",
    "services/search-service"
)

if ($Target -ne "all") {
    $tsProjects = $tsProjects | Where-Object { $_ -match $Target }
    $goProjects = $goProjects | Where-Object { $_ -match $Target }
}

foreach ($proj in $tsProjects) {
    if (Test-Path "$proj/package.json") {
        Write-Host "`n---> Running Test Coverage for $proj (Node/Jest)..." -ForegroundColor Yellow
        npm --prefix $proj test -- --coverage --passWithNoTests
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[FAIL] Coverage failed in $proj" -ForegroundColor Red
            $failed++
        } else {
            Write-Host "[PASS] Coverage collected for $proj" -ForegroundColor Green
        }
    }
}

foreach ($proj in $goProjects) {
    if (Test-Path "$proj/go.mod") {
        Write-Host "`n---> Running Test Coverage for $proj (Go)..." -ForegroundColor Yellow
        Push-Location $proj
        try {
            go test -cover ./...
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[FAIL] Coverage failed in $proj" -ForegroundColor Red
                $failed++
            } else {
                Write-Host "[PASS] Coverage collected for $proj" -ForegroundColor Green
            }
        } finally {
            Pop-Location
        }
    }
}

Write-Host "`n==========================================" -ForegroundColor Cyan
if ($failed -gt 0) {
    Write-Host " Coverage run finished with $failed failure(s)" -ForegroundColor Red
    exit 1
} else {
    Write-Host " All test coverage collected successfully!" -ForegroundColor Green
    exit 0
}

