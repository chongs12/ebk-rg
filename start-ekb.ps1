# start-ekb.ps1
param (
    [switch]$SkipBuild = $false
)

$ErrorActionPreference = "Stop"
$ProjectRoot = $PSScriptRoot
$K8sDir = Join-Path $ProjectRoot "k8s"

Write-Host "[START] Starting Enterprise Knowledge Base (EKB) on Minikube..." -ForegroundColor Green

# 1. 启动 Minikube（如果未运行）
Write-Host "[INFO] Checking Minikube status..." -ForegroundColor Cyan
$MinikubeRunning = $true
minikube status -p minikube 2>$null | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ACTION] Starting Minikube with custom resources..." -ForegroundColor Yellow
    minikube start --cpus=4 --memory=8192 --disk-size=20g --driver=docker
    $MinikubeRunning = $false
} else {
    Write-Host "[OK] Minikube is already running." -ForegroundColor DarkGray
}

# 2. 启用 Ingress
Write-Host "[INFO] Enabling Ingress addon..." -ForegroundColor Cyan
minikube addons enable ingress | Out-Null

# 3. 配置当前 Shell 使用 Minikube 的 Docker daemon
Write-Host "[INFO] Switching Docker context to Minikube..." -ForegroundColor Cyan
& minikube -p minikube docker-env --shell powershell | Invoke-Expression

# 4. 构建镜像（除非跳过）
if (-not $SkipBuild) {
    $Services = @(
        @{ Name = "auth";      Path = "cmd/auth/Dockerfile" },
        @{ Name = "document";  Path = "cmd/document/Dockerfile" },
        @{ Name = "vector";    Path = "cmd/vector/Dockerfile" },
        @{ Name = "query";     Path = "cmd/query/Dockerfile" },
        @{ Name = "gateway";   Path = "cmd/gateway/Dockerfile" }
    )

    foreach ($svc in $Services) {
        $dockerfilePath = Join-Path $ProjectRoot $svc.Path
        if (-not (Test-Path $dockerfilePath)) {
            Write-Error "Dockerfile not found: $dockerfilePath"
        }
        $imageTag = "ekb/$($svc.Name):latest"
        Write-Host "[BUILD] Building $imageTag ..." -ForegroundColor Cyan
        docker build -t $imageTag -f $dockerfilePath $ProjectRoot
    }
} else {
    Write-Host "[SKIP] Skipping image builds (use -SkipBuild)." -ForegroundColor DarkGray
}

# 5. 应用 Kubernetes manifests in order
Write-Host "[APPLY] Applying Kubernetes manifests from '$K8sDir'..." -ForegroundColor Cyan

# 确保目录存在
if (-not (Test-Path $K8sDir)) {
    Write-Error "K8s config directory not found: $K8sDir"
}

# 按逻辑顺序应用
$Manifests = @(
    "configmap.yaml",
    "secret.yaml",
    "persistent-volume.yaml",
    "jaeger.yaml",
    "deployment.yaml",
    "service.yaml",
    "ingress.yaml"
)

foreach ($manifest in $Manifests) {
    $path = Join-Path $K8sDir $manifest
    if (Test-Path $path) {
        Write-Host "  → Applying $manifest" -ForegroundColor Gray
        kubectl apply -f $path
    } else {
        Write-Host "  [WARN] Skipped (not found): $manifest" -ForegroundColor Yellow
    }
}

# 6. 输出使用提示
$ip = minikube ip
Write-Host ""
Write-Host "[DONE] EKB deployment complete!" -ForegroundColor Green
Write-Host "[URL] Gateway (NodePort): http://$ip`:30080" -ForegroundColor White
Write-Host "[URL] Gateway (Ingress):  http://ekb.local (add '192.168.49.2 ekb.local' to your hosts file)" -ForegroundColor White
Write-Host "[URL] Jaeger UI:         http://$ip`:16686" -ForegroundColor White
Write-Host ""
Write-Host "[TIP] Run './stop-ekb.ps1' to clean up resources." -ForegroundColor DarkGray