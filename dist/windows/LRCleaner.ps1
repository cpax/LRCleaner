# define variables
$global:port = 8501 # LogRhythm port that api is listening on.  Default is 8501.
$global:configFilePath = "LRCleanerConfig.json" # Config file path - we are storing hostname and api key here.
$global:excludedLogSources = @(".*logrhythm.*", ".*Open.*Collector.*", ".*echo.*", ".*AI Engine.*", "LogRhythm.*") # Excluded log sources - we are excluding any log sources that match these regex patterns.

# Define the log file path
$logFilePath = "scsc.log" # Log file path - we are logging all the output to this file.

# define the functions

# Function to log messages and output to std out and log file.
function Log-Message {
    param (
        [string]$message
    )
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "$timestamp - $message"
    Write-Host $logMessage
    Add-Content -Path $logFilePath -Value $logMessage
}

# Function to log messages and output to log file only.
function Log-Only {
    param (
        [string]$message
    )
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "$timestamp - $message"
    Add-Content -Path $logFilePath -Value $logMessage
}

#function to load the config.  If the config file is not found, we will build it.  Config is stored in LRCleanerConfig.json.
function Load-Config {
    Log-Only "######################## BEGINNING CONFIG LOAD #############################"
    if (Test-Path $global:configFilePath) {
        Log-Message "Config file found. Loading configuration..."
        $config = Get-Content $global:configFilePath | ConvertFrom-Json
        $secureApiKey = $config.apiKey | ConvertTo-SecureString
        $global:apiKey = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto([System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureApiKey))
        
        # Debugging output to check the content of the config
        Log-Only "Config content: $($config | ConvertTo-Json)"
        
        $global:apihostname = $config.hostname
        
        # Debugging output to check the hostname value
        Log-Only "Hostname from config: $global:apihostname"
    } else {
        Log-Message "Config file not found. Building configuration..."
        # Call the function to build the configuration
        Build-Config
    }
    return
}
# Function to display help
function Show-Help {
    Write-Host "Usage: .\LRCleaner.ps1 [-test] [-apply] [-help]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -test    Run the script in test mode."
    Write-Host "  -apply   Apply the changes."
    Write-Host "  -help    Display this help message."
}
#function to apply the changes
function Apply-Section {
    Write-Host "Applying changes..."

    Write-Host "Please select the CSV file containing the log source IDs and host IDs to be retired."
    Write-Host "This file should be in the same directory as the LRCleaner.ps1 file."
    Write-Host "The file should be a CSV file with two columns, 'id' and 'hostId'."
    Write-Host "The 'id' column should contain the log source IDs and the 'hostId' column should contain the host IDs."
    Write-Host ""

    $openFileDialog = New-Object System.Windows.Forms.OpenFileDialog
    $openFileDialog.InitialDirectory = [System.Environment]::GetFolderPath("MyDocuments")
    $openFileDialog.filter = "CSV files (*.csv)|*.csv"
    $result = $openFileDialog.ShowDialog()
    if ($result -eq [System.Windows.Forms.DialogResult]::OK) {
        $selectedFilePath = $openFileDialog.FileName
        Log-Message "Selected file path: $selectedFilePath"
    } else {
        Write-Host "No file selected."
        return
    }

    $idsAndHostIds = @()
    if ($selectedFilePath) {
        $csvData = Import-Csv -Path $selectedFilePath
        foreach ($row in $csvData) {
            $idsAndHostIds += [PSCustomObject]@{
                id = $row.id
                hostId = $row.hostId
            }
        }
        Write-Host "Loaded IDs and Host IDs from CSV file."
    } else {
        Write-Host "No file selected. No IDs and Host IDs loaded."
        return
    }

    $uniqueHostIds = @()

    foreach ($entry in $idsAndHostIds) {
        $id = $entry.id

        Log-Message "Updating log source ID $id..."
        # Pull the JSON object for the log source
        $getUri = "https://${global:apihostname}:${global:port}/lr-admin-api/logsources/$id"
        $headers = @{
            "Authorization" = "Bearer $global:apiKey"
            "Content-Type" = "application/json"
        }
        try {
            $logSource = Invoke-RestMethod -Uri $getUri -Method Get -Headers $headers -ContentType "application/json" -SkipCertificateCheck
        } catch {
            Write-Host "Failed to retrieve log source ID $id. Error: $_"
            continue
        }

        # Update the name and record status in the JSON object
        $logSource.name += " Retired by LRCleaner"
        $logSource.recordStatus = "Retired"
        # Convert the updated JSON object to a string
        $updateBody = $logSource | ConvertTo-Json

        # Update the log source
        $updateUri = "https://${global:apihostname}:${global:port}/lr-admin-api/logsources/$id"
        try {
            $response = Invoke-RestMethod -Uri $updateUri -Method Put -Headers $headers -Body $updateBody -ContentType "application/json" -SkipCertificateCheck
            Write-Host "Updated log source ID $id successfully."
        } catch {
            Write-Host "Failed to update log source ID $id. Error: $_"
        }
    }

    #update the hosts
    foreach ($entry in $idsAndHostIds) {
        $hostId = $entry.hostId

        # Query the log sources endpoint again
        $queryUri = "https://${global:apihostname}:${global:port}/lr-admin-api/hosts/$hostId"
        $response = Invoke-RestMethod -Uri $queryUri -Method Get -Headers $headers -ContentType "application/json" -SkipCertificateCheck
        $filteredResponse = $response | Where-Object { $_.hostId -eq $hostId }
        try {
            $response = Invoke-RestMethod -Uri $queryUri -Method Get -Headers $headers -ContentType "application/json" -SkipCertificateCheck
            if ($response.Count -eq 0) {
                Write-Host "Host ID $hostId is ready for retirement."
                $hostsReadyForRetirement += $hostId
            } else {
                Write-Host "Host ID $hostId is not ready for retirement."
                $hostsNotReadyForRetirement += $hostId
            }
        } catch {
            Write-Host "Failed to query log sources for host ID $hostId. Error: $_"
        }

        # Add hostId to uniqueHostIds array if not already present
        if (-not $uniqueHostIds.Contains($hostId)) {
            $uniqueHostIds += $hostId
        }
    }

    Write-Host "Unique Host IDs: $($uniqueHostIds | ConvertTo-Json)"
    return
}
#function to build the config
function Build-Config {
    Log-Message "Building configuration..."
    Add-Type -AssemblyName System.Windows.Forms

    $form = New-Object System.Windows.Forms.Form
    $form.Text = "Configuration"
    $form.Size = New-Object System.Drawing.Size(300,200)
    $form.StartPosition = "CenterScreen"

    $apiKeyLabel = New-Object System.Windows.Forms.Label
    $apiKeyLabel.Location = New-Object System.Drawing.Point(10,20)
    $apiKeyLabel.Size = New-Object System.Drawing.Size(280,20)
    $apiKeyLabel.Text = "API Key:"
    $form.Controls.Add($apiKeyLabel)

    $apiKeyTextBox = New-Object System.Windows.Forms.TextBox
    $apiKeyTextBox.Location = New-Object System.Drawing.Point(10,40)
    $apiKeyTextBox.Size = New-Object System.Drawing.Size(260,20)
    $form.Controls.Add($apiKeyTextBox)

    $hostnameLabel = New-Object System.Windows.Forms.Label
    $hostnameLabel.Location = New-Object System.Drawing.Point(10,70)
    $hostnameLabel.Size = New-Object System.Drawing.Size(280,20)
    $hostnameLabel.Text = "Hostname:"
    $form.Controls.Add($hostnameLabel)

    $hostnameTextBox = New-Object System.Windows.Forms.TextBox
    $hostnameTextBox.Location = New-Object System.Drawing.Point(10,90)
    $hostnameTextBox.Size = New-Object System.Drawing.Size(260,20)
    $form.Controls.Add($hostnameTextBox)

    $buttonPanel = New-Object System.Windows.Forms.Panel
    $buttonPanel.Location = New-Object System.Drawing.Point(0,150)
    $buttonPanel.Size = New-Object System.Drawing.Size(300,50)
    $buttonPanel.Dock = "Bottom"
    $form.Controls.Add($buttonPanel)

    $okButton = New-Object System.Windows.Forms.Button
    $okButton.Location = New-Object System.Drawing.Point(75,10)
    $okButton.Size = New-Object System.Drawing.Size(75,23)
    $okButton.Text = "Submit"
    $okButton.DialogResult = [System.Windows.Forms.DialogResult]::OK
    $form.AcceptButton = $okButton
    $buttonPanel.Controls.Add($okButton)

    $cancelButton = New-Object System.Windows.Forms.Button
    $cancelButton.Location = New-Object System.Drawing.Point(160,10) # Adjusted to add space
    $cancelButton.Size = New-Object System.Drawing.Size(75,23)
    $cancelButton.Text = "Cancel"
    $cancelButton.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
    $form.CancelButton = $cancelButton
    $buttonPanel.Controls.Add($cancelButton)
    $form.Topmost = $true

    $result = $form.ShowDialog()

    if ($result -eq [System.Windows.Forms.DialogResult]::Cancel) {
        return
    }

    $global:apiKey = $apiKeyTextBox.Text
    $global:apihostname = $hostnameTextBox.Text

    if ($apiKey.Length -lt 400) {
        [System.Windows.Forms.MessageBox]::Show("This does not appear to be a valid LogRhythm API Key.", "Error", [System.Windows.Forms.MessageBoxButtons]::OK, [System.Windows.Forms.MessageBoxIcon]::Error)
        $form.ShowDialog()
        return
    }

    if ([string]::IsNullOrWhiteSpace($hostname)) {
        [System.Windows.Forms.MessageBox]::Show("Hostname cannot be blank.", "Error", [System.Windows.Forms.MessageBoxButtons]::OK, [System.Windows.Forms.MessageBoxIcon]::Error)
        $form.ShowDialog()
        return
    }

    $secureApiKey = ConvertTo-SecureString -String $apiKey -AsPlainText -Force | ConvertFrom-SecureString
    
    $config = [PSCustomObject]@{
        apiKey = $secureApiKey
        apihostname = $global:apihostname
    } | ConvertTo-Json | Set-Content "LRCleanerConfig.json" -Force
    return
}
#function to select the last log message date
function lastLogMessageDate {
    Log-Message "Selecting last log message date..."
    Add-Type -AssemblyName System.Windows.Forms
    $datePicker = New-Object System.Windows.Forms.MonthCalendar
    $datePicker.MaxSelectionCount = 1
    $datePicker.Dock = [System.Windows.Forms.DockStyle]::Fill

    $form = New-Object System.Windows.Forms.Form
    $form.ClientSize = New-Object System.Drawing.Size(400, 400) # Adjust the size as needed
    $form.Controls.Add($datePicker)

    $submitButton = New-Object System.Windows.Forms.Button
    $submitButton.Location = New-Object System.Drawing.Point(150, 350)
    $submitButton.Size = New-Object System.Drawing.Size(100, 23)
    $submitButton.Text = "Submit"
    $submitButton.DialogResult = [System.Windows.Forms.DialogResult]::OK
    $form.AcceptButton = $submitButton
    $form.Controls.Add($submitButton)

    $result = $form.ShowDialog()

    if ($result -eq [System.Windows.Forms.DialogResult]::OK) {
        $global:selectedDate = $datePicker.SelectionStart
        # We only need the formatted date for the API call, but the API call doesn't return logs in an expected manner.
        # $global:formattedSelectedDate = $global:selectedDate.ToString("yyyy-MM-ddTHH:mm:ss.fffZ") -replace ":", "%3A"
        Log-Message "Selected date: $global:selectedDate"
    } else {
        Log-Message "Date selection was cancelled."
    }
}

# function to perform a test run
function Test-Section {
    $offset = 0
    $count = 1000
    $allResponses = @()

    do {
        $baseUri = "https://${global:apihostname}:${global:port}/lr-admin-api/logsources?recordStatus=active&offset=$offset&count=$count&dir=ascending&orderBy=maxLogDate"
        $headers = @{
            "Authorization" = "Bearer $global:apiKey"
            "Content-Type" = "application/json"
        }

        Log-Message "Constructed URI: '$baseUri'"  # Debugging output
        $response = Invoke-RestMethod -Uri $baseUri -Method Get -Headers $headers -SkipCertificateCheck
        $response | ConvertTo-Json -Depth 5 | Set-Content "LRCleanerResponse.json" # Debugging output

        $responseCount = $response.Count
        Log-Message "Response count: $responseCount"

        $allResponses += $response

        $offset += $count
    } while ($responseCount -eq $count)

    $totalResponses = $allResponses.Count
    Log-Message "This is the number of responses we got back from the API call before any filtering is applied."
    Log-Message "Total responses: $totalResponses"

    # Remove any log sources that have a maxLogDate greater than the selected date
    $dateFilteredResponses = $allResponses | Where-Object {
        [datetime]$_.maxLogDate -le $selectedDate
      }
    $dateFilteredResponses | ConvertTo-Json -Depth 5 | Set-Content "LRCleanerDateFilteredResponse.json" # Debugging output

    $dateFilteredResponsesCount = $dateFilteredResponses.Count
    Log-Message "Total number of sources after filtering out sources with a maxLogDate greater than the selected date: $dateFilteredResponsesCount"

    # Remove any log sources that match the excludedLogSources array or if the name starts with "echo"
    $sourceFilteredResponses = $dateFilteredResponses | Where-Object {
        $sourceType = $_.logSourceType.name
        $logSourceName = $_.name
        $maxLogDate = [datetime]$_.maxLogDate
        $recordStatus = $_.recordStatus
        $matchFound = $false
        
        # Skip already retired log sources
        if ($recordStatus -eq "Retired") {
            $matchFound = $true
        }
        
        foreach ($exclusion in $global:excludedLogSources) {
            if ($sourceType -imatch $exclusion) {
                $matchFound = $true
                break
            }
        }
        if ($logSourceName -like "echo*") {
            $matchFound = $true
        }
        !$matchFound
    }
    $sourceFilteredResponses | ConvertTo-Json -Depth 5 | Set-Content "LRCleanerSourceFilteredResponse.json" # Debugging output

    $sourceFilteredResponsesCount = $sourceFilteredResponses.Count
    Log-Message "You can control which log sources are filtered out by editing the excludedLogSources array.  By default Open Collector, Echo and LogRhythm are excluded."
    Log-Message "Total number of sources after filtering out excluded sources: $sourceFilteredResponsesCount"

    $logSourceDetails = $sourceFilteredResponses | Select-Object @{Name="ID";Expression={$_.id}}, @{Name="HostID";Expression={$_.host.id}}, @{Name="HostName";Expression={$_.host.name}}, @{Name="MaxLogDate";Expression={$_.maxLogDate}}
    Write-Host "Log source details: $($logSourceDetails | ConvertTo-Json)"

    $pingResults = @()
    foreach ($logSource in $logSourceDetails) {
        $pingResult = New-Object PSObject -Property @{
            ID = $logSource.ID
            HostID = $logSource.HostID
            HostName = $logSource.HostName
            PingResult = if ((Test-Connection -ComputerName $logSource.HostName -Count 1).Count -eq 1) {"Success"} else {"Failure"}
            MaxLogDate = $logSource.MaxLogDate
        }
        $pingResults += $pingResult
    }

    # Logging output
    # Write-Host "Ping results: $($pingResults)"

    # Export the results to a CSV file, notify the user and open explorer
    $pingResults | Select-Object ID, HostID, HostName, PingResult, MaxLogDate | Export-Csv -Path "LRCleanerAllResults.csv" -NoTypeInformation
   
    # Filter the ping results to only include successful pings
    $successfulPings = $pingResults | Where-Object { $_.PingResult -eq "Success" }
    $successfulPings | Select-Object ID, HostID, HostName, PingResult, MaxLogDate | Export-Csv -Path "TroubleShoot.csv" -NoTypeInformation

    $failedPings = $pingResults | Where-Object { $_.PingResult -eq "Failure" }
    $failedPings | Select-Object ID, HostID, HostName, PingResult, MaxLogDate | Export-Csv -Path "UnPingable.csv" -NoTypeInformation
    [System.Windows.Forms.MessageBox]::Show("Testing of inactive log sources completed. Results saved to three csv files, All results, TroubleShoot and UnPingable. We recommend troubleshooting log collection for any hosts in the TroubleShoot file.", "Notification", [System.Windows.Forms.MessageBoxButtons]::OK, [System.Windows.Forms.MessageBoxIcon]::Information)
    Start-Process explorer $(Get-Location).Path
    return
}

# Main Script Logic


# Check if running PowerShell 7 or higher
if ($PSVersionTable.PSVersion.Major -lt 7) {
    Log-Message "This script requires PowerShell 7 or higher."
    return
}

# Process the command line arguments
if ($args -contains '-help') {
    Show-Help
    return
}

if ($args -contains '-test') {
    Log-Only "********************* BEGINNING TEST RUN *********************"
    Log-Only "Running in test mode..."
    Load-Config
    lastLogMessageDate
    Test-Section
    Log-Only "********************** END OF TEST RUN ***********************"
    return
}

if ($args -contains '-apply') {
    Log-Only "********************* BEGINNING APPLY RUN *********************"
    Log-Only "Running in apply mode..."
    Load-Config
    Apply-Section
    Log-Only "********************** END OF APPLY RUN ***********************"
    return
}

# If no valid arguments are provided, show help
Show-Help

