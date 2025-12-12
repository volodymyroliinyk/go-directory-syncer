#!/bin/bash
INSTANCE_NAME=$1

# --- DEFINE PATH DEFINITION LOGIC HERE ---
# The simplest option: using the case operator to specify paths by instance name
case $INSTANCE_NAME in
    projectA)
        SOURCE="/home/$USER/data/source_A"
        DESTINATION="/mnt/backup/dest_A"
        SYNC_STRATEGY="mirror"
        ;;
    projectB)
        SOURCE="/home/$USER/docs/source_B"
        DESTINATION="/mnt/cloud/dest_B"
        SYNC_STRATEGY="merge"
        ;;
    # Add other sync pairs here
    *)
        echo "Error: Unknown syncer instance name: $INSTANCE_NAME" >&2
        exit 1
        ;;
esac

# Running your Go script with defined paths
exec /usr/local/bin/go-directory-syncer \
    --source-directory="$SOURCE" \
    --destination-directory="$DESTINATION" \
    --strategy="$SYNC_STRATEGY"