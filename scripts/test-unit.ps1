# Run Unit Tests across all services and packages
param(
    [string]$Target = "all"
)

$ErrorActionPreference = "Continue"
$failed = 0

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host " Running Unit Tests (Target: $Target)" -ForegroundColor Cyan
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
        Write-Host "`n---> Running Unit Tests for $proj (Node/Jest)..." -ForegroundColor Yellow
        npm --prefix $proj test -- --coverage=false --passWithNoTests
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[FAIL] Unit tests failed in $proj" -ForegroundColor Red
            $failed++
        } else {
            Write-Host "[PASS] Unit tests passed in $proj" -ForegroundColor Green
        }
    }
}

foreach ($proj in $goProjects) {
    if (Test-Path "$proj/go.mod") {
        Write-Host "`n---> Running Unit Tests for $proj (Go)..." -ForegroundColor Yellow
        Push-Location $proj
        try {
            go test ./...
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[FAIL] Unit tests failed in $proj" -ForegroundColor Red
                $failed++
            } else {
                Write-Host "[PASS] Unit tests passed in $proj" -ForegroundColor Green
            }
        } finally {
            Pop-Location
        }
    }
}

Write-Host "`n==========================================" -ForegroundColor Cyan
if ($failed -gt 0) {
    Write-Host " Unit tests finished with $failed failure(s)" -ForegroundColor Red
    exit 1
} else {
    Write-Host " All unit tests passed successfully!" -ForegroundColor Green
    exit 0
}

