#!/bin/bash
#
# Build and run all services.
#
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)
PROJECT_ROOT=$(dirname "$SCRIPT_DIR")

GO_BINARY_NAME="go-directory-syncer"
GO_SOURCE_FILE="$GO_BINARY_NAME.go"

WRAPPER_SCRIPT_SOURCE_NAME="go-directory-syncer-wrapper-private.sh" # use the private version
WRAPPER_SCRIPT_TARGET_NAME="go-directory-syncer-wrapper.sh" # use the private version
SYSTEMD_SERVICE_TARGET_NAME="go-directory-syncer@.service"

# Installation paths
BIN_PATH="/usr/local/bin"
SYSTEMD_PATH="/etc/systemd/system"

# === CHANGE ONLY HERE WHEN ADDING A NEW PROJECT ===
# All instances that need to be started and managed
#PROJECTS_TO_MANAGE=("projectA" "projectB" "projectC")
PROJECTS_TO_MANAGE=("projectA" "projectB")
# =======================================================

# Check for root rights for sudo
if [ "$(id -u)" -ne 0 ]; then
    echo "This script requires root privileges to install files to /usr/local/bin and /etc/systemd/system."
    echo "Please run with sudo or confirm when prompted."
fi

# --- 2. BUILD AND COPY THE GO EXECUTE ---
echo "--- 1. Building Go binary ($GO_SOURCE_FILE) ---"
# Build the binary in the root directory
(cd "$PROJECT_ROOT" && \
    echo "Resolving dependencies (go mod tidy)..." && \
    /usr/local/go/bin/go mod tidy && \
    echo "Compiling Go binary..." && \
    /usr/local/go/bin/go build -o "$GO_BINARY_NAME" "$GO_SOURCE_FILE")

# --- 5a. STOP SERVICE (BEFORE COPYING) ---
for INSTANCE_NAME in "${PROJECTS_TO_MANAGE[@]}"; do
#  INSTANCE="projectA"
  SERVICE_TEMPLATE_NAME="$SYSTEMD_SERVICE_TARGET_NAME"
  SERVICE_INSTANCE="${SERVICE_TEMPLATE_NAME/.service/$INSTANCE_NAME.service}"
  echo "Attempting to stop active service instance: $SERVICE_INSTANCE"
  # Stop, ignore errors if the service is not running
  sudo systemctl stop "$SERVICE_INSTANCE" 2>/dev/null
done

echo "--- 2. Copying binary to $BIN_PATH ---"
sudo cp "$PROJECT_ROOT/$GO_BINARY_NAME" "$BIN_PATH/$GO_BINARY_NAME"
sudo chmod +x "$BIN_PATH/$GO_BINARY_NAME"
echo "Go binary installed to $BIN_PATH/$GO_BINARY_NAME"

# --- 3. COPYING AND CONFIGURING THE WRAPPER SCRIPT ---
echo "--- 3. Copying and setting up wrapper script ($WRAPPER_SCRIPT_SOURCE_NAME) ---"
WRAPPER_SOURCE_PATH="$SCRIPT_DIR/$WRAPPER_SCRIPT_SOURCE_NAME"
WRAPPER_TARGET_PATH="$BIN_PATH/$WRAPPER_SCRIPT_TARGET_NAME"

sudo cp "$WRAPPER_SOURCE_PATH" "$WRAPPER_TARGET_PATH"
sudo chmod +x "$WRAPPER_TARGET_PATH"
echo "Wrapper script installed to $WRAPPER_TARGET_PATH"

# --- 4. COPYING AND CONFIGURING THE SERVICE SYSTEMD ---
echo "--- 4. Installing Systemd service template ($SYSTEMD_SERVICE_TEMPLATE_NAME) ---"
SERVICE_SOURCE_PATH="$PROJECT_ROOT/service/$SYSTEMD_SERVICE_TEMPLATE_NAME"
SERVICE_TARGET_PATH="$SYSTEMD_PATH/$SYSTEMD_SERVICE_TARGET_NAME"
# Copy the service template with a new, shorter name
sudo cp "$SERVICE_SOURCE_PATH" "$SERVICE_TARGET_PATH"
echo "Systemd template installed to $SERVICE_TARGET_PATH"


for INSTANCE_NAME in "${PROJECTS_TO_MANAGE[@]}"; do
  SERVICE_INSTANCE="${SERVICE_TEMPLATE_NAME/.service/$INSTANCE_NAME.service}"
  echo "--- 5. Reloading Systemd and starting $SERVICE_INSTANCE ---"
  # Reload configuration
  sudo systemctl daemon-reload
  # Starting the first instance
  sudo systemctl start "$SERVICE_INSTANCE"
  # Enable autorun
  sudo systemctl enable "$SERVICE_INSTANCE"

  echo "--- Installation Complete ---"
  echo "To manage, use the instance name, e.g.: go-directory-syncer@$INSTANCE"
  echo "Status check: sudo systemctl status $SERVICE_INSTANCE"
  echo "Logs: sudo journalctl -u $SERVICE_INSTANCE -f"
done