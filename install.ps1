# Windows installation script for RLAMA
Write-Host "
 ____  _       _    __  __    _    
|  _ \| |     / \  |  \/  |  / \   
| |_) | |    / _ \ | |\/| | / _ \  
|  _ <| |___/ ___ \| |  | |/ ___ \ 
|_| \_\_____/_/   \_\_|  |_/_/   \_\
                                  
Retrieval-Augmented Language Model Adapter for Windows
"

# Determine architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$binaryName = "rlama_windows_$arch.exe"

# Create installation directories
$dataDir = "$env:USERPROFILE\.rlama"
$installDir = "$env:LOCALAPPDATA\RLAMA"

Write-Host "Installing RLAMA..."
Write-Host "Downloading RLAMA for Windows $arch..."

# Create directories if they don't exist
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

# Download the binary
$downloadUrl = "https://github.com/dontizi/rlama/releases/latest/download/$binaryName"
$outputPath = "$installDir\rlama.exe"

try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $outputPath
    
    # Add to PATH if not already there
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$installDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "User")
        Write-Host "Added RLAMA to your PATH. You may need to restart your terminal."
    }
    
    Write-Host "RLAMA has been successfully installed to $outputPath!"
    Write-Host "You can now use RLAMA by running the 'rlama' command."
} catch {
    Write-Host "Error downloading RLAMA: $_"
    exit 1
}