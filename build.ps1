# PowerShell build script for minio2rustfs migration tool

param(
    [Parameter(Position=0)]
    [string]$Command = "help",
    [string]$Version = "dev"
)

$BinaryName = "minio2rustfs"
$MainPath = ".\cmd\main.go"
$BuildDir = "build"
$BuildTime = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"

function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Show-Help {
    Write-ColorOutput Blue "Available commands:"
    Write-Output "  build   - Build the binary"
    Write-Output "  clean   - Clean build artifacts"
    Write-Output "  test    - Run tests"
    Write-Output "  fmt     - Format code"
    Write-Output "  vet     - Vet code"
    Write-Output "  run     - Build and run the application"
    Write-Output "  deps    - Download dependencies"
    Write-Output "  all     - Clean, format, vet, test, and build"
    Write-Output ""
    Write-ColorOutput Blue "Example usage:"
    Write-Output "  .\build.ps1 build"
    Write-Output "  .\build.ps1 test"
    Write-Output "  .\build.ps1 -Command build -Version v1.0.0"
}

function Invoke-Build {
    Write-ColorOutput Blue "Building $BinaryName..."
    if (!(Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }
    
    $ldflags = "-X main.version=$Version -X main.buildTime=$BuildTime"
    & go build -ldflags $ldflags -o "$BuildDir\$BinaryName.exe" $MainPath
    
    if ($LASTEXITCODE -eq 0) {
        Write-ColorOutput Green "Build completed: $BuildDir\$BinaryName.exe"
    } else {
        Write-ColorOutput Red "Build failed!"
        exit 1
    }
}

function Invoke-Clean {
    Write-ColorOutput Yellow "Cleaning..."
    & go clean
    if (Test-Path $BuildDir) {
        Remove-Item -Recurse -Force $BuildDir
    }
    Write-ColorOutput Green "Clean completed"
}

function Invoke-Test {
    Write-ColorOutput Blue "Running tests..."
    & go test -v ./...
}

function Invoke-Format {
    Write-ColorOutput Blue "Formatting code..."
    & go fmt ./...
}

function Invoke-Vet {
    Write-ColorOutput Blue "Vetting code..."
    & go vet ./...
}

function Invoke-Run {
    Invoke-Build
    if ($LASTEXITCODE -eq 0) {
        Write-ColorOutput Blue "Running $BinaryName..."
        & ".\$BuildDir\$BinaryName.exe" --help
    }
}

function Invoke-Deps {
    Write-ColorOutput Blue "Downloading dependencies..."
    & go mod download
    & go mod tidy
    Write-ColorOutput Green "Dependencies updated"
}

function Invoke-All {
    Invoke-Clean
    Invoke-Format
    Invoke-Vet
    Invoke-Test
    Invoke-Build
}

# Main execution
switch ($Command.ToLower()) {
    "build" { Invoke-Build }
    "clean" { Invoke-Clean }
    "test" { Invoke-Test }
    "fmt" { Invoke-Format }
    "vet" { Invoke-Vet }
    "run" { Invoke-Run }
    "deps" { Invoke-Deps }
    "all" { Invoke-All }
    default { Show-Help }
}