#!/bin/bash

# Azure Hello World Function Deploy Script
set -e

echo "=== Azure Python Hello World Function Deployer ==="

# Set variables
RESOURCE_GROUP="jp-presale-test"
LOCATION="eastus2"  # Change to your preferred region
APP_NAME="python-hello-$(date +%s | sha256sum | base64 | head -c 8 | tr '[:upper:]' '[:lower:]')"
STORAGE_NAME="jaytest001"
PYTHON_VERSION="3.11"

# Login to Azure (if not already logged in)
echo "Checking Azure login..."
az account show >/dev/null 2>&1 || {
    echo "Please login to Azure..."
    az login
}

# Create Resource Group
#echo "Creating resource group: $RESOURCE_GROUP"
#az group create --name $RESOURCE_GROUP --location $LOCATION >/dev/null 2>&1 || {
#    echo "Resource group already exists or error occurred."
#}

# Create Storage Account
#echo "Creating storage account: $STORAGE_NAME"
#az storage account create \
#  --name $STORAGE_NAME \
#  --location $LOCATION \
#  --resource-group $RESOURCE_GROUP \
#  --sku Standard_LRS \
#  --kind StorageV2 >/dev/null 2>&1


#zip -r deployment.zip . \
#  -x '.venv/*' \
#  -x '.git/*' \
#  -x '__pycache__/*' \
#  -x '*.env' \
#  -x 'local.settings.json' \
#  -x '*.log'

# Create Function App
echo "Creating Function App: $APP_NAME"
az functionapp create \
  --name $APP_NAME \
  --storage-account $STORAGE_NAME \
  --consumption-plan-location $LOCATION \
  --resource-group $RESOURCE_GROUP \
  --os-type Linux \
  --runtime python \
  --runtime-version $PYTHON_VERSION \
  --functions-version 4 >/dev/null 2>&1

# Deploy the function code
echo "Deploying function code..."
zip -r deployment.zip ./* >/dev/null 2>&1
az functionapp deployment source config-zip \
  --resource-group $RESOURCE_GROUP \
  --name $APP_NAME \
  --src ./deployment.zip >/dev/null 2>&1

# Clean up zip file
rm -f deployment.zip

# Get the function URL
FUNCTION_URL="https://$(az functionapp show --name $APP_NAME --resource-group $RESOURCE_GROUP --query defaultHostName -o tsv)/api/hello"

echo ""
echo "=== Deployment Complete! ==="
echo "Function App Name: $APP_NAME"
echo "Resource Group: $RESOURCE_GROUP"
echo ""
echo "Your Hello World function is ready!"
echo ""
echo "Test your function:"
echo "  GET Request: curl '$FUNCTION_URL?name=Azure'"
echo "  POST Request: curl -X POST '$FUNCTION_URL' -H 'Content-Type: application/json' -d '{\"name\": \"Python\"}'"
echo ""
echo "Management:"
echo "  Azure Portal: https://portal.azure.com/#@/resource/subscriptions/{subscription-id}/resourceGroups/$RESOURCE_GROUP/providers/Microsoft.Web/sites/$APP_NAME/overview"
echo ""
echo "To delete all resources when done:"
echo "  az group delete --name $RESOURCE_GROUP --yes --no-wait"
