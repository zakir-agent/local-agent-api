#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

BIN="$ROOT/local-agent-api"
PID_FILE="$ROOT/logs/local-agent-api.pid"
RUNENV_FILE="$ROOT/logs/local-agent-api.runenv"
NOHUP_LOG="$ROOT/logs/daemon.log"

PORT="${PORT:-8080}"
MODEL="${MODEL:-composer-2-fast}"
AGENT_CLI="${AGENT_CLI:-cursor}"

usage() {
  echo "Usage: $0 {start|stop|status|restart}"
  echo ""
  echo "Environment (optional):"
  echo "  PORT        listen port (default: 8080)"
  echo "  MODEL       -model flag (default: composer-2-fast)"
  echo "  AGENT_CLI   claude | cursor (default: cursor)"
  echo ""
  echo "Examples:"
  echo "  $0 start"
  echo "  AGENT_CLI=claude $0 start"
}

is_running_pid() {
  local pid="$1"
  [[ -n "$pid" && "$pid" =~ ^[0-9]+$ ]] || return 1
  kill -0 "$pid" 2>/dev/null
}

read_runenv_display() {
  _disp_port="$PORT"
  _disp_model="$MODEL"
  _disp_agent="$AGENT_CLI"
  [[ -f "$RUNENV_FILE" ]] || return 0
  local line key val
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" == *'='* ]] || continue
    key="${line%%=*}"
    val="${line#*=}"
    case "$key" in
      PORT) _disp_port="$val" ;;
      MODEL) _disp_model="$val" ;;
      AGENT_CLI) _disp_agent="$val" ;;
    esac
  done <"$RUNENV_FILE"
}

cmd_start() {
  mkdir -p logs

  if [[ -f "$PID_FILE" ]]; then
    local old
    old="$(tr -d '[:space:]' <"$PID_FILE")"
    if is_running_pid "$old"; then
      echo "already running (pid $old); $0 stop first."
      exit 1
    fi
    rm -f "$PID_FILE" "$RUNENV_FILE"
  fi

  echo "building..."
  go build -o "$BIN" .

  echo "---- $(date "+%Y-%m-%dT%H:%M:%S%z") start ----" >>"$NOHUP_LOG"
  echo "starting (nohup), port=$PORT agent-cli=$AGENT_CLI model=$MODEL"
  nohup "$BIN" -port "$PORT" -model "$MODEL" -agent-cli "$AGENT_CLI" >>"$NOHUP_LOG" 2>&1 &
  echo $! >"$PID_FILE"
  {
    printf 'PORT=%s\n' "$PORT"
    printf 'MODEL=%s\n' "$MODEL"
    printf 'AGENT_CLI=%s\n' "$AGENT_CLI"
  } >"$RUNENV_FILE"
  echo "pid $(cat "$PID_FILE")"
  echo "log: $NOHUP_LOG"
}

cmd_stop() {
  if [[ ! -f "$PID_FILE" ]]; then
    echo "not running (no pid file)"
    rm -f "$RUNENV_FILE"
    return 0
  fi

  local pid
  pid="$(tr -d '[:space:]' <"$PID_FILE")"
  if ! is_running_pid "$pid"; then
    echo "stale pid file (pid ${pid:-?}), removing"
    rm -f "$PID_FILE" "$RUNENV_FILE"
    return 0
  fi

  echo "sending SIGTERM to $pid..."
  kill -TERM "$pid" || true

  local i
  for ((i = 0; i < 50; i++)); do
    if ! is_running_pid "$pid"; then
      break
    fi
    sleep 0.2
  done

  if is_running_pid "$pid"; then
    echo "still running, sending SIGKILL"
    kill -9 "$pid" 2>/dev/null || true
  fi

  rm -f "$PID_FILE" "$RUNENV_FILE"
  echo "stopped"
}

cmd_status() {
  if [[ ! -f "$PID_FILE" ]]; then
    echo "not running"
    return 1
  fi
  local pid
  pid="$(tr -d '[:space:]' <"$PID_FILE")"
  if ! is_running_pid "$pid"; then
    echo "not running (stale pid ${pid:-?})"
    rm -f "$PID_FILE" "$RUNENV_FILE"
    return 1
  fi
  read_runenv_display
  echo "running pid $pid (port $_disp_port, agent-cli=$_disp_agent, model=$_disp_model)"
  return 0
}

cmd_restart() {
  cmd_stop || true
  sleep 0.5
  cmd_start
}

case "${1:-}" in
  start) cmd_start ;;
  stop) cmd_stop ;;
  status) cmd_status ;;
  restart) cmd_restart ;;
  -h | --help | help | "")
    usage
    [[ -n "${1:-}" ]] || exit 1
    exit 0
    ;;
  *)
    usage
    exit 1
    ;;
esac
