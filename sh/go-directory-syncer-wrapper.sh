#!/bin/bash
INSTANCE_NAME=$1

# --- DEFINE PATH DEFINITION LOGIC HERE ---
# The simplest option: using the case operator to specify paths by instance name
case $INSTANCE_NAME in
    projectA)
        SYNC_STRATEGY="mirror"
        SOURCE="/home/$USER/data/source_A"
        DESTINATION="/mnt/backup/dest_A"
        ;;
    projectB)
        SYNC_STRATEGY="merge"
        SOURCE="/home/$USER/docs/source_B"
        DESTINATION="/mnt/cloud/dest_B"
        ;;
    # Add other sync pairs here
    *)
        echo "Error: Unknown syncer instance name: $INSTANCE_NAME" >&2
        exit 1
        ;;
esac

# Running your Go script with defined paths
exec /usr/local/bin/go-directory-syncer \
    --strategy="$SYNC_STRATEGY" \
    --source-directory="$SOURCE" \
    --destination-directory="$DESTINATION"