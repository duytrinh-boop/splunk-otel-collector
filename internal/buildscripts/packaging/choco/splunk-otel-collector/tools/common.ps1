$installation_path = "${env:PROGRAMFILES}\Splunk\OpenTelemetry Collector"
$program_data_path = "${env:PROGRAMDATA}\Splunk\OpenTelemetry Collector"
$config_path = "$program_data_path\"

$service_name = "splunk-otel-collector"

# whether the service is running
function service_running([string]$name) {
    return ((Get-CimInstance -ClassName win32_service -Filter "Name = '$name'" | Select Name, State).State -Eq "Running")
}

# whether the service is installed
function service_installed([string]$name) {
    return ((Get-CimInstance -ClassName win32_service -Filter "Name = '$name'" | Select Name, State).Name -Eq "$name")
}

# start the service if it's stopped
function start_service([string]$name=$service_name, [string]$config_path=$config_path, [int]$max_attempts=3, [int]$timeout=60) {
    if (!(service_running -name "$name")) {
        if (Test-Path -Path $config_path) {
            for ($i=1; $i -le $max_attempts; $i++) {
                try {
                    Start-Service -Name "$name"
                    break
                } catch {
                    $err = $_.Exception.Message
                    $message = @"
An error occurred while trying to start the $name service:
$err
Please check the system and application logs.
"@
                    if ($i -eq $max_attempts) {
                        throw "$message"
                    }
                    Write-Warning "$message"
                    Start-Sleep -Seconds 10
                }
            }
            wait_for_service -name "$name" -timeout $timeout
        } else {
            throw "$config_path does not exist and is required to start the $name service"
        }
    }
}

# stop the service if it's running
function stop_service([string]$name, [int]$max_attempts=3) {
    if (service_running -name "$name") {
        for ($i=1; $i -le $max_attempts; $i++) {
            try {
                Stop-Service -Name "$name"
                break
            } catch {
                $err = $_.Exception.Message
                $message = @"
An error occurred while trying to stop the $name service:
$err
Please check the system and application logs.
"@
                if ($i -eq $max_attempts) {
                    throw "$message"
                }
                Write-Warning "$message"
                Start-Sleep -Seconds 10
            }
        }
        wait_for_service_stop -name "$name"
    }
}

# remove registry entries created by the splunk-otel-collector service
function remove_otel_registry_entries() {
    try {
        if (Test-Path "HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\splunk-otel-collector"){
            Remove-Item "HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\splunk-otel-collector"
        }
    } catch {
        $err = $_.Exception.Message
        $message = "
        unable to remove registry entries at HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\splunk-otel-collector
        $err
        "
        throw "$message"
    }
}

function update_registry([string]$path, [string]$name, [string]$value) {
    write-host "Updating $path for $name..."
    Set-ItemProperty -path "$path" -name "$name" -value "$value"
}

# wait for the service to start
function wait_for_service([string]$name=$service_name, [int]$timeout=60) {
    $startTime = Get-Date
    while (!(service_running -name "$name")){
        if ((New-TimeSpan -Start $startTime -End (Get-Date)).TotalSeconds -gt $timeout){
            throw @"
Timed out waiting for the $name service to be running.
Please check the system and application logs.
"@
        }
        # give windows a second to synchronize service status
        Start-Sleep -Seconds 1
    }
}

# wait for the service to stop
function wait_for_service_stop([string]$name=$service_name, [int]$timeout=60) {
    $startTime = Get-Date
    while (service_running -name "$name"){
        if ((New-TimeSpan -Start $startTime -End (Get-Date)).TotalSeconds -gt $timeout){
            throw @"
Timed out waiting for the $name service to be stopped.
Please check the system and application logs.
"@
        }
        # give windows a second to synchronize service status
        Start-Sleep -Seconds 1
    }
}

# check that we're not running with a restricted execution policy
function check_policy() {
    $executionPolicy  = (Get-ExecutionPolicy)
    $executionRestricted = ($executionPolicy -eq "Restricted")
    if ($executionRestricted) {
        throw @"
Your execution policy is $executionPolicy, this means you will not be able import or use any scripts including modules.
To fix this change you execution policy to something like RemoteSigned.
        PS> Set-ExecutionPolicy RemoteSigned
For more information execute:
        PS> Get-Help about_execution_policies
"@
    }
}

function install_msi([string]$path) {
    Write-Host "Installing $path ..."
    $startTime = Get-Date
    $proc = (Start-Process msiexec.exe -Wait -PassThru -ArgumentList "/qn /norestart /i `"$path`"")
    if ($proc.ExitCode -ne 0 -and $proc.ExitCode -ne 3010) {
        $err = "The installer failed with error code ${proc.ExitCode}."
        try {
            $events = (Get-WinEvent -ProviderName "MsiInstaller" | Where-Object { $_.TimeCreated -ge $startTime })
            if ($events) {
                $err += ($events | Format-List | Out-String)
            }
        } catch {
            $err += "`r`nPlease check the system and application logs."
            continue
        }
        throw "$err"
    }
    Write-Host "- Done"
}

$ErrorActionPreference = 'Stop'; # stop on all errors
