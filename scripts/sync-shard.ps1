Get-ChildItem -Path "services" -Directory | ForEach-Object {
    $target = Join-Path $_.FullName "node_modules/@delivery-micro/shard/dist"
    if (Test-Path $target) {
        Copy-Item -Path "packages/ts/dist/*" -Destination $target -Recurse -Force
        Write-Host "Updated shard in $($_.Name)" -ForegroundColor Green
    }
}
