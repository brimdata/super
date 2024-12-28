#!/bin/bash -xv
set -euo pipefail
export RUNNING_ON_AWS_EC2=""

# If we can detect we're running on an AWS EC2 m6idn.2xlarge instance, we'll
# treat it as a scratch host, installing all needed software and using the
# local SSD for best I/O performance.
if command -v dmidecode && [ "$(sudo dmidecode --string system-uuid | cut -c1-3)" == "ec2" ] && [ "$(TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600") && curl -H "X-aws-ec2-metadata-token: $TOKEN" http://169.254.169.254/latest/meta-data/instance-type)" == "m6idn.2xlarge" ]; then

  export RUNNING_ON_AWS_EC2=true

  sudo apt-get -y update
  sudo apt-get -y upgrade
  sudo apt-get -y install make gcc unzip hyperfine

  # Prepare local SSD for best I/O performance
  sudo fdisk -l /dev/nvme1n1
  sudo mkfs.ext4 -E discard -F /dev/nvme1n1
  sudo mount /dev/nvme1n1 /mnt
  sudo chown ubuntu:ubuntu /mnt
  sudo chmod 777 /mnt
  echo 'export TMPDIR="/mnt/tmpdir"' >> "$HOME"/.profile
  mkdir /mnt/tmpdir

  # Install DuckDB
  if ! command -v duckdb > /dev/null 2>&1; then
    curl -L -O https://github.com/duckdb/duckdb/releases/download/v1.1.3/duckdb_cli-linux-amd64.zip
    unzip duckdb_cli-linux-amd64.zip
    sudo mv duckdb /usr/local/bin
  fi

  # Install Rust
  curl -L -O https://static.rust-lang.org/dist/rust-1.82.0-x86_64-unknown-linux-gnu.tar.xz
  tar xf rust-1.82.0-x86_64-unknown-linux-gnu.tar.xz
  sudo rust-1.82.0-x86_64-unknown-linux-gnu/install.sh
  # shellcheck disable=SC2016
  echo 'export PATH="$PATH:$HOME/.cargo/bin"' >> "$HOME"/.profile

  # Install DataFusion CLI
  if ! command -v datafusion-cli > /dev/null 2>&1; then
    cargo install datafusion-cli
  fi

  # Install Go
  if ! command -v go > /dev/null 2>&1; then
    curl -L -O https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
    rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
    # shellcheck disable=SC2016
    echo 'export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"' >> "$HOME"/.profile
    source "$HOME"/.profile
  fi

  # Install SuperDB
  if ! command -v super > /dev/null 2>&1; then
    git clone https://github.com/brimdata/super.git
    cd super
    make install
  fi

  cd scripts/super-cmd-perf

  # Install ClickHouse
  if ! command -v clickhouse-client > /dev/null 2>&1; then
    sudo apt-get install -y apt-transport-https ca-certificates curl gnupg
    curl -fsSL 'https://packages.clickhouse.com/rpm/lts/repodata/repomd.xml.key' | sudo gpg --dearmor -o /usr/share/keyrings/clickhouse-keyring.gpg
    echo "deb [signed-by=/usr/share/keyrings/clickhouse-keyring.gpg] https://packages.clickhouse.com/deb stable main" | sudo tee \
        /etc/apt/sources.list.d/clickhouse.list
    sudo apt-get update
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y clickhouse-server clickhouse-client
    sudo cp clickhouse-storage.xml /etc/clickhouse-server/config.d
    sudo systemctl stop clickhouse-server
    sudo systemctl disable clickhouse-server.service
  fi

fi

rundir="$(date +%F_%T)"
mkdir "$rundir"
report="$rundir/report_$rundir.md"

echo -e "|**Software**|**Version**|\n|-|-|" | tee -a "$report"
for software in super duckdb datafusion-cli clickhouse
do
  if ! command -v $software > /dev/null; then
    echo "error: \"$software\" not found in PATH"
    exit 1
  fi
  echo "|$software|$($software --version)|" | tee -a "$report"
done
echo >> "$report"

# Prepare the test data
./prep-data.sh "$rundir"

# Run the queries and generate the summary report
./run-queries.sh "$rundir"

if [ -n "$RUNNING_ON_AWS_EC2" ]; then
  mv "$HOME/runlog.txt" "$rundir"
fi
