#!/bin/bash
set -e

cd "$(dirname "$0")"

echo "Building checkin..."
go build -o checkin .

mkdir -p "$HOME/bin"
mv checkin "$HOME/bin/checkin"

echo "Installed to ~/bin/checkin"

# Check for config
if [ ! -f "$HOME/.config/checkin/config.json" ]; then
    echo ""
    echo "Next step: create config file"
    echo "  mkdir -p ~/.config/checkin"
    echo '  cat > ~/.config/checkin/config.json << EOF'
    echo '  {'
    echo '    "client_id": "YOUR_APP_CLIENT_ID",'
    echo '    "tenant_id": "YOUR_TENANT_ID"'
    echo '  }'
    echo '  EOF'
    echo ""
    echo "Run ./setup.sh to register an Azure AD app, or see README.md for manual steps."
fi
