#!/bin/bash
#
# Stop and disable services.
#
PROJECTS_TO_MANAGE=("projectA" "projectB")

for INSTANCE_NAME in "${PROJECTS_TO_MANAGE[@]}"; do
    SERVICE_INSTANCE="${SERVICE_TEMPLATE_NAME/.service/$INSTANCE_NAME.service}"

  sudo systemctl stop "$SERVICE_INSTANCE"
  sudo systemctl disable "$SERVICE_INSTANCE"
done