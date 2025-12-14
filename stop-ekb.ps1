# stop-ekb.ps1
$K8sDir = Join-Path $PSScriptRoot "k8s"

Write-Host "[STOP] Stopping Enterprise Knowledge Base (K8s resources only)..." -ForegroundColor Red
Write-Host "[NOTE] Minikube and built images are preserved. No rebuild needed!" -ForegroundColor Yellow

if (Test-Path $K8sDir) {
    Write-Host "[INFO] Deleting Kubernetes resources..." -ForegroundColor Cyan
    kubectl delete -f $K8sDir --ignore-not-found=$true
} else {
    Write-Host "[WARN] k8s directory not found. Skipping delete." -ForegroundColor Magenta
}

Write-Host ""
Write-Host "[DONE] Cleanup complete." -ForegroundColor Green
Write-Host "[TIP] To restart quickly, run: .\start-ekb.ps1 -SkipBuild" -ForegroundColor DarkGray
Write-Host "[WARN] Only run 'minikube delete' if you want to reset everything (loses images)." -ForegroundColor Red