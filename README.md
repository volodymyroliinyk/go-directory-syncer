# go-directory-syncer

## Reliable Go-directory synchronizer with file tracking

`go-directory-syncer` - is a high-performance, multithreaded command-line tool written in Go. It is designed for one-way
directory synchronization, running as a persistent service (daemon) by using `fsnotify` to instantly track file system
changes.

This tool is ideal for fast local sync and backup, especially on Linux/Ubuntu systems.

## Key features

* **Synchronization strategies:** Support for two operating modes: **Mirror** (mirror copying with deletion) and **Merge
  ** (merge without deletion).
* **Instant synchronization:** Uses the `fsnotify` library to react immediately to changes, avoiding constant polling of
  the filesystem.
* **Concurrency:** Uses Goroutines to copy and delete files in parallel, ensuring maximum speed.
* **Metadata preservation:** Synchronizes files by preserving access rights (`chmod`) and modification times (
  `Chtimes`), which mimics the behavior of `cp --preserve=all`.
* **Reliability:** Designed to run as a `Systemd` service to provide automatic start and process monitoring.

## Strategies

The script supports the `--strategy` option, which defines the synchronization behavior.

| Strategy   | Parameter                     | Description                                               | Behavior regarding redundant files in `destination`      |
|:-----------|:------------------------------|:----------------------------------------------------------|:---------------------------------------------------------|
| **Mirror** | `--strategy=mirror` (default) | **Mirroring.** Makes `destination` identical to `source`. | **Removes** files and directories not found in `source`. |
| **Merge**  | `--strategy=merge`            | **Copy/Merge.** Updates only changed files.               | **Retains** files and directories not found in `source`. |

-----

## Install and run (Ubuntu/Linux)

The best way to run `go-directory-syncer` is to compile and configure it as a Systemd template service.

### 1\. Compilation of the executable file

1.1) Navigate to the `sh/` directory, copy `go-directory-syncer-wrapper.sh`(example) to
`go-directory-syncer-wrapper-private.sh`, and modify last one for your needs, you can create project"N", how many you
need.

1.2) Navigate to the `sh/` directory, copy `build.sh`(example) to `build-private.sh`, and modify just one line
`PROJECTS_TO_MANAGE=("projectA" "projectB" "projectC" ... )`, add all your projects names as list.

1.3) You can use any `"YourPersonalProjectName"` instead of `"projectA"`

1.4) Navigate to the `sh/` directory and run `build-private.sh`.

```bash
# Go to the root/sh directory of the project
cd /path/to/go-directory-syncer/sh;

# Run the script with sudo privileges to install to system directories
sudo bash build-private.sh;
```

> **What does `build-private.sh' do?**
>
> 1. Resolves Go dependencies (`go mod tidy`).
> 2. Compiles `go-directory-syncer.go` into a `/usr/local/bin/go-directory-syncer` binary.
> 3. Copies the wrapper script (`go-directory-syncer-wrapper-private.sh`) to
     `/usr/local/bin/go-directory-syncer-wrapper.sh`.
> 4. Installs the Systemd service template (`go-directory-syncer@.service`).

### 2\. Script-Wrapping settings

Because Systemd uses templates, you define synchronization options in a wrapper file. Open the installed script:

```bash
# Example file
sudo nano /usr/local/bin/go-directory-syncer-wrapper.sh;

# Your file with original projects configs
sudo nano /usr/local/bin/go-directory-syncer-wrapper-private.sh;
```

Add or modify sync pairs to suit your needs, including your preferred strategy:

```bash
case $INSTANCE_NAME in
    projectA)
        SYNC_STRATEGY="mirror" # <-- MIRROR COPY WITH DELETE
        SOURCE="/home/$USER/projects/data_in1"
        DESTINATION="/home/$USER/projects/data_out_merged1"
        ;;
    projectB)
        SYNC_STRATEGY="merge" # <-- MERGER WITHOUT DELETE
        SOURCE="/home/$USER/projects/data_in2"
        DESTINATION="/home/$USER/projects/data_out_merged2"
        ;;
    *)
        # ... (error logic)
        ;;
esac

exec /usr/local/bin/go-directory-syncer \
    --strategy="$SYNC_STRATEGY" \
    --source-directory="$SOURCE" \
    --destination-directory="$DESTINATION"
```

### 3\. Starting the service Systemd

Use the name of your project (`projectA`, `projectB`, etc.) as the instance name for the service:

```bash
# 1. Reload the Systemd configuration
sudo systemctl daemon-reload;

# 2. Start a service instance (eg for projectA)
sudo systemctl start go-directory-syncer@projectA.service;
sudo systemctl start go-directory-syncer@projectB.service; 

# 3. Enable automatic startup at system boot
sudo systemctl enable go-directory-syncer@projectA.service;
sudo systemctl enable go-directory-syncer@projectB.service;
```

## Monitoring and logging

Use `journalctl` to view service logs in real time:

```bash
# Check service status
sudo systemctl status go-directory-syncer@projectA.service;
sudo systemctl status go-directory-syncer@projectB.service;

# View logs for a specific instance
sudo journalctl -u go-directory-syncer@projectA.service -f;
sudo journalctl -u go-directory-syncer@projectB.service -f;
```

-----

## Testing

The project includes Go unit tests (`go_directory_syncer_test.go`), which check the main synchronization logic and
guarantee the correct operation of the **Mirror** and **Merge** strategies.

To run the tests, go to the root directory and run:

```bash
go test -v go-directory-syncer.go go_directory_syncer_test.go;
```

-----

## Licence

This project is distributed under the MIT license. See the `LICENSE` file for details.