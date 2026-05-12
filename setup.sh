#!/bin/bash
set -e

# Register an Azure AD app for checkin using the Azure CLI.
# Requires: az cli installed and logged in (az login)
#
# Permission GUIDs (Microsoft Graph, delegated):
#   e1fe6dd8-ba31-4d61-89e7-88639da4683d = User.Read
#   024d486e-b451-40bb-833d-3e66d98c5c73 = Mail.ReadWrite
#   e383f46e-2787-4529-855e-0681a7b0be68 = Mail.Send
#   465a38f9-76ea-45b9-9f34-9e8b0d4b0b42 = Calendars.Read
#   9ff7295e-131b-4d94-90e1-69fde507ac11 = Chat.ReadWrite

if ! command -v az &> /dev/null; then
    echo "Azure CLI (az) not found."
    echo ""
    echo "You can install it (https://learn.microsoft.com/en-us/cli/azure/install-azure-cli-linux)"
    echo "or register the app manually in the Azure portal:"
    echo ""
    echo "  1. Go to https://portal.azure.com → Azure Active Directory → App registrations"
    echo "  2. New registration → Name: 'checkin' → Accounts in this org only"
    echo "  3. Under Authentication → Add platform → Mobile and desktop"
    echo "     → Check 'https://login.microsoftonline.com/common/oauth2/nativeclient'"
    echo "     → Enable 'Allow public client flows' → Save"
    echo "  4. Under API permissions → Add:"
    echo "       Microsoft Graph (Delegated):"
    echo "       - User.Read"
    echo "       - Mail.ReadWrite"
    echo "       - Mail.Send"
    echo "       - Calendars.Read"
    echo "       Optional (requires admin consent):"
    echo "       - Chat.ReadWrite"
    echo "  5. Copy Application (client) ID and Directory (tenant) ID into:"
    echo "     ~/.config/checkin/config.json"
    echo ""
    exit 1
fi

echo "Checking Azure CLI login..."
az account show > /dev/null 2>&1 || { echo "Please run 'az login' first."; exit 1; }

TENANT_ID=$(az account show --query tenantId -o tsv)
echo "Using tenant: $TENANT_ID"

# Base permissions: User.Read, Mail.ReadWrite, Mail.Send, Calendars.Read
PERMISSIONS='[{
    "resourceAppId": "00000003-0000-0000-c000-000000000000",
    "resourceAccess": [
        {"id": "e1fe6dd8-ba31-4d61-89e7-88639da4683d", "type": "Scope"},
        {"id": "024d486e-b451-40bb-833d-3e66d98c5c73", "type": "Scope"},
        {"id": "e383f46e-2787-4529-855e-0681a7b0be68", "type": "Scope"},
        {"id": "465a38f9-76ea-45b9-9f34-9e8b0d4b0b42", "type": "Scope"}
    ]
}]'

ENABLE_TEAMS=false

read -p "Include Teams chat support? (requires admin consent) [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    PERMISSIONS='[{
        "resourceAppId": "00000003-0000-0000-c000-000000000000",
        "resourceAccess": [
            {"id": "e1fe6dd8-ba31-4d61-89e7-88639da4683d", "type": "Scope"},
            {"id": "024d486e-b451-40bb-833d-3e66d98c5c73", "type": "Scope"},
            {"id": "e383f46e-2787-4529-855e-0681a7b0be68", "type": "Scope"},
            {"id": "465a38f9-76ea-45b9-9f34-9e8b0d4b0b42", "type": "Scope"},
            {"id": "9ff7295e-131b-4d94-90e1-69fde507ac11", "type": "Scope"}
        ]
    }]'
    ENABLE_TEAMS=true
fi

echo "Registering app..."
APP_ID=$(az ad app create \
    --display-name "checkin" \
    --is-fallback-public-client true \
    --public-client-redirect-uris "https://login.microsoftonline.com/common/oauth2/nativeclient" \
    --required-resource-accesses "$PERMISSIONS" \
    --query appId -o tsv)

echo "App registered: $APP_ID"

mkdir -p "$HOME/.config/checkin"
cat > "$HOME/.config/checkin/config.json" << EOF
{
    "client_id": "$APP_ID",
    "tenant_id": "$TENANT_ID",
    "enable_teams": $ENABLE_TEAMS
}
EOF

echo "Config written to ~/.config/checkin/config.json"

if [ "$ENABLE_TEAMS" = true ]; then
    echo ""
    echo "Note: Chat.ReadWrite requires admin consent."
    echo "Ask your IT admin to grant consent in the Azure portal, or run:"
    echo "  az ad app permission admin-consent --id $APP_ID"
fi

echo ""
echo "You can now run: checkin"
