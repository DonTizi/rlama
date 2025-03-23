#!/bin/bash

# Installation script for RLAMA dependencies
# This script attempts to install the necessary tools for text extraction
# and reranking with BGE

echo "Installing dependencies for RLAMA..."

# Operating system detection
OS=$(uname -s)
echo "Detected operating system: $OS"

# Function to check if a program is installed
is_installed() {
  command -v "$1" >/dev/null 2>&1
}

# macOS
if [ "$OS" = "Darwin" ]; then
  echo "Installing dependencies for macOS..."
  
  # Check if Homebrew is installed
  if ! is_installed brew; then
    echo "Homebrew not found. Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  fi
  
  # Install tools
  echo "Installing text extraction tools..."
  brew install poppler  # For pdftotext
  brew install tesseract  # For OCR
  brew install tesseract-lang  # Additional languages for Tesseract
  
  # Python and tools
  if ! is_installed pip3; then
    brew install python
  fi
  
  pip3 install pdfminer.six docx2txt xlsx2csv
  
# Linux
elif [ "$OS" = "Linux" ]; then
  echo "Installing dependencies for Linux..."
  
  # Try apt-get (Debian/Ubuntu)
  if is_installed apt-get; then
    echo "Package manager apt-get detected"
    sudo apt-get update
    sudo apt-get install -y poppler-utils tesseract-ocr tesseract-ocr-fra python3-pip
    sudo apt-get install -y catdoc unrtf
  
  # Try yum (CentOS/RHEL)
  elif is_installed yum; then
    echo "Package manager yum detected"
    sudo yum update
    sudo yum install -y poppler-utils tesseract tesseract-langpack-fra python3-pip
    sudo yum install -y catdoc
  
  # Try pacman (Arch Linux)
  elif is_installed pacman; then
    echo "Package manager pacman detected"
    sudo pacman -Syu
    sudo pacman -S poppler tesseract tesseract-data-fra python-pip
  
  # Try zypper (openSUSE)
  elif is_installed zypper; then
    echo "Package manager zypper detected"
    sudo zypper refresh
    sudo zypper install poppler-tools tesseract-ocr python3-pip
  
  else
    echo "No known package manager detected. Please install the dependencies manually."
  fi
  
  # Install Python packages
  pip3 install --user pdfminer.six docx2txt xlsx2csv

# Windows (via WSL)
elif [[ "$OS" == MINGW* ]] || [[ "$OS" == MSYS* ]] || [[ "$OS" == CYGWIN* ]]; then
  echo "Windows system detected."
  echo "It is recommended to use WSL (Windows Subsystem for Linux) for better performance."
  echo "You can install the dependencies manually:"
  echo "1. Install Python: https://www.python.org/downloads/windows/"
  echo "2. Install Python packages: pip install pdfminer.six docx2txt xlsx2csv FlagEmbedding torch transformers"
  echo "3. For OCR, install Tesseract: https://github.com/UB-Mannheim/tesseract/wiki"
  
  # Try to install Python packages with pip in Windows
  if is_installed pip; then
    echo "Installing Python dependencies on Windows..."
    pip install --user pdfminer.six docx2txt xlsx2csv
    pip install --user -U FlagEmbedding torch transformers
  elif is_installed pip3; then
    echo "Installing Python dependencies on Windows..."
    pip3 install --user pdfminer.six docx2txt xlsx2csv
    pip3 install --user -U FlagEmbedding torch transformers
  fi
fi

# Install common Python dependencies
echo "Installing common Python dependencies..."
if is_installed pip3; then
  pip3 install --user pdfminer.six docx2txt xlsx2csv
  echo "Installing dependencies for BGE reranker..."
  pip3 install --user -U FlagEmbedding torch transformers
elif is_installed pip; then
  pip install --user pdfminer.six docx2txt xlsx2csv
  echo "Installing dependencies for BGE reranker..."
  pip install --user -U FlagEmbedding torch transformers
else
  echo "⚠️ Pip is not installed. Cannot install Python dependencies."
  echo "Please install pip then run: pip install -U FlagEmbedding pdfminer.six docx2txt xlsx2csv"
fi

echo "Installation completed!"
echo ""
echo "To use the BGE reranker, run: rlama update-reranker [rag-name]"
echo "This will configure your RAG to use the BAAI/bge-reranker-v2-m3 model for reranking." 