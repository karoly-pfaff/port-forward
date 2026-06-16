param(
  [string]$OutputDir = ".\build\windows",
  [switch]$Clean
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$outputPath = if ([System.IO.Path]::IsPathRooted($OutputDir)) {
  $OutputDir
} else {
  Join-Path $repoRoot $OutputDir
}
$outputPath = [System.IO.Path]::GetFullPath($outputPath)

$serverJsPath = Join-Path $outputPath "server.js"
$serviceExePath = Join-Path $outputPath "service.exe"
$cliExePath = Join-Path $outputPath "portier.exe"
$webOutputPath = Join-Path $outputPath "web"
$bundleWorkPath = Join-Path $outputPath "_bundle"
$bundleCjsPath = Join-Path $bundleWorkPath "server.cjs"
# The packaged single-file Node fallback (server.js) bundles the NestJS server
# (sources/index.ts). NestJS lazily requires optional transports it never uses for
# a plain HTTP app, so those are marked external for esbuild ($bundleExternals
# below). The Go service remains the preferred packaged runtime. See server/readme.md.
$bundleEntryPath = Join-Path $repoRoot "server\sources\index.ts"
$bundleExternals = @(
  "--external:@nestjs/microservices",
  "--external:@nestjs/websockets/socket-module",
  "--external:@nestjs/microservices/microservices-module",
  "--external:class-transformer/storage"
)

function Get-LocalToolPath {
  param([string]$Name)

  $cmdPath = Join-Path $repoRoot "node_modules\.bin\$Name.cmd"
  if (Test-Path $cmdPath) { return $cmdPath }

  $plainPath = Join-Path $repoRoot "node_modules\.bin\$Name"
  if (Test-Path $plainPath) { return $plainPath }

  throw "Required local tool '$Name' was not found. Run 'npm install' first."
}

function Invoke-CommandInRepo {
  param([string]$FilePath, [string[]]$Arguments)

  Push-Location $repoRoot
  try {
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
      throw "Command failed with exit code $LASTEXITCODE`: $FilePath $($Arguments -join ' ')"
    }
  } finally {
    Pop-Location
  }
}

if ($Clean -and (Test-Path $outputPath)) {
  Remove-Item -LiteralPath $outputPath -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $bundleWorkPath | Out-Null

Write-Host "Building shared package..."
Invoke-CommandInRepo "npm.cmd" @("run", "build:shared")

Write-Host "Building server package..."
Invoke-CommandInRepo "npm.cmd" @("run", "build:server")

Write-Host "Building client package..."
Invoke-CommandInRepo "npm.cmd" @("run", "build:client")

$esbuild = Get-LocalToolPath "esbuild"

Write-Host "Bundling server as server.js..."
Invoke-CommandInRepo $esbuild (@(
  $bundleEntryPath,
  "--bundle",
  "--minify",
  "--platform=node",
  "--format=cjs",
  "--target=node22",
  "--outfile=$bundleCjsPath"
) + $bundleExternals)
Copy-Item -LiteralPath $bundleCjsPath -Destination $serverJsPath -Force

# The bundle is CommonJS, but the package dir has no package.json and the repo root
# is "type": "module" — without a marker Node would load server.js as ESM and fail on
# its require() calls. Mark the package dir as CommonJS so the Node fallback runs.
Set-Content -LiteralPath (Join-Path $outputPath "package.json") -Value '{ "type": "commonjs" }' -Encoding ascii

Write-Host "Generating OpenAPI document..."
Invoke-CommandInRepo "npm.cmd" @("run", "generate:apidoc")

Write-Host "Copying OpenAPI document into package..."
Invoke-CommandInRepo "npm.cmd" @("run", "copy:apidoc:release", "-w", "server", "--", $outputPath)

Write-Host "Building Go service for Windows..."
Push-Location (Join-Path $repoRoot "service")
try {
  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"
  & go build -o $serviceExePath ./sources
  if ($LASTEXITCODE -ne 0) {
    throw "Go build failed with exit code $LASTEXITCODE"
  }
} finally {
  Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
  Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
  Pop-Location
}

Write-Host "Building Go CLI for Windows..."
Push-Location (Join-Path $repoRoot "tools\cli")
try {
  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"
  & go build -o $cliExePath ./sources
  if ($LASTEXITCODE -ne 0) {
    throw "Go CLI build failed with exit code $LASTEXITCODE"
  }
} finally {
  Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
  Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
  Pop-Location
}

Write-Host "Copying web UI..."
if (Test-Path $webOutputPath) {
  Remove-Item -LiteralPath $webOutputPath -Recurse -Force
}
Copy-Item -LiteralPath (Join-Path $repoRoot "client\build") -Destination $webOutputPath -Recurse -Force

@"
Portier Windows Portable Package
=================================

This portable archive contains the Portier runtime files only. It does not
install OS services. Use the Inno Setup installer or the install scripts from
the Portier repository to set up a Windows Service or scheduled task.

Files in this package:
  portier.exe   CLI -- control a running Portier service from the terminal
  service.exe   Native Go runtime (preferred; start as Windows Service)
  server.js     Node.js fallback runtime (requires Node.js)
  web\          Built React management UI
  api\openapi.json  OpenAPI 3 description of the /api surface
  readme.txt    This file

CLI usage (requires a running Portier service):
  .\portier.exe runtime
  .\portier.exe list
  .\portier.exe status
  .\portier.exe diagnose <id|name>
  .\portier.exe config export --out rules.json
  .\portier.exe diagnostics export --out diagnostics.json

  The CLI does not start or install the service by itself.
  Default management URL: http://127.0.0.1:47831

Native service (preferred):
  .\service.exe --service --config "C:\path\to\rules.json" --host 127.0.0.1 --port 47831 --static-dir ".\web"

Node server (fallback, requires Node.js):
  node .\server.js --service --config "C:\path\to\rules.json" --host 127.0.0.1 --port 47831 --static-dir ".\web"

Options:
  --config <path>     Path to rules.json (required; not bundled in this archive)
  --host <addr>       Management host (default: 127.0.0.1)
  --port <port>       Management port (default: 47831)
  --static-dir <dir>  Path to web UI directory (use ".\web" from this directory)

Default management URL:
  http://127.0.0.1:47831

Config (rules.json) is external and must be provided. A new empty rules.json is
created automatically if the path does not exist when the service starts.

Machine-wide install (Inno Setup installer, requires Inno Setup 6):
  npm run build:release:current    (from the Portier repository)
  Installs to %ProgramFiles%\Portier\ with config at %ProgramData%\Portier\rules.json.

Manual install scripts in the repository:
  scripts\windows\service\install-service.ps1 -Scope Machine  (Administrator required)
  scripts\windows\service\install-service.ps1 -Scope User     (no Administrator required)

Forwarded listen ports may need Windows Firewall inbound rules when listening on 0.0.0.0.
Do not run both a Machine and User install on the same port at the same time.
"@ | Set-Content -LiteralPath (Join-Path $outputPath "readme.txt") -Encoding ascii

Remove-Item -LiteralPath $bundleWorkPath -Recurse -Force

Write-Host "Windows package created at $outputPath"
Write-Host "  CLI        : $cliExePath"
Write-Host "  Go service : $serviceExePath"
Write-Host "  Node server: $serverJsPath"
Write-Host "  OpenAPI    : $(Join-Path $outputPath 'api\openapi.json')"
Write-Host "  Web UI     : $webOutputPath"
