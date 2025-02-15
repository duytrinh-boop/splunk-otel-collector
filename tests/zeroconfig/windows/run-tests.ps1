# Runs E2E tests for zero-confing for Windows deployments

# This script update Firewall rules and needs elevation to work correctly.
#Requires -RunAsAdministrator

$ErrorActionPreference = 'Stop'

$repo_root = (git rev-parse --show-toplevel)
$testdata_path = Join-Path $repo_root tests/zeroconfig/windows/testdata/
$docker_setup_path = Join-Path $testdata_path docker-setup/

if (!(Test-Path $docker_setup_path)) {
    New-Item $testdata_path -ItemType Directory -Value docker-setup
}

# Copy files required to build the docker images.

if (!(Test-Path (Join-Path $docker_setup_path install.ps1) -PathType Leaf)) {
    Copy-Item $repo_root/internal/buildscripts/packaging/installer/install.ps1 $docker_setup_path
}

if (!(Test-Path (Join-Path $docker_setup_path splunk-otel-collector-*.msi) -PathType Leaf)) {
    $version = (git describe --tags --abbrev=0).SubString(1)
    $collector_msi = Join-Path  $docker_setup_path splunk-otel-collector-$version-amd64.msi
    Invoke-WebRequest -Uri https://github.com/signalfx/splunk-otel-collector/releases/download/v$version/splunk-otel-collector-$version-amd64.msi -OutFile $collector_msi -UseBasicParsing
}

try {
    # Build the .NET applications
    Push-Location $repo_root/tests/zeroconfig/windows/testdata/apps/
    nuget restore
    msbuild .\AspNet.WebApi.NetFramework\AspNet.WebApi.NetFramework.csproj /p:Configuration=Release /p:Platform=AnyCPU /p:OutputPath=..\bin

    try {
        # Setup the required Firewall rule
        New-NetFirewallRule -DisplayName 'zc-iis-test' -Direction Inbound -LocalAddress 10.1.1.1 -LocalPort 4318 -Protocol TCP -Action Allow -Profile Any

        # Build the docker compose
        Set-Location $repo_root/tests/zeroconfig/windows/testdata/
        docker compose build

        # Run the tests
        Set-Location $repo_root/tests/zeroconfig/windows/
        go test -timeout 5m -tags zeroconfig -v
    }
    finally {
        Remove-NetFirewallRule -DisplayName 'zc-iis-test'
    }
}
finally {
    Pop-Location
}
