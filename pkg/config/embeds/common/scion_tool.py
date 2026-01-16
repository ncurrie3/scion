import json
import os
import sys
import tempfile
from datetime import datetime

HOME = os.path.expanduser("~")
SCION_JSON_PATH = os.path.join(HOME, "agent-info.json")
AGENT_LOG_PATH = os.path.join(HOME, "agent.log")

def log_event(state, message):
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    with open(AGENT_LOG_PATH, "a") as f:
        f.write(f"{timestamp} [{state}] {message}\n")

    if "user" in message.lower() and state not in ["WAITING_FOR_INPUT", "COMPLETED"]:
        # Special case: don't reset to ACTIVE if it's the system auto-continue prompt
        if "System: Please continue" not in message:
            update_status("ACTIVE", session=True)

def update_status(status, session=False):
    data = {}
    if os.path.exists(SCION_JSON_PATH):
        try:
            with open(SCION_JSON_PATH, "r") as f:
                data = json.load(f)
        except Exception as e:
            log_event("ERROR", f"Failed to read {SCION_JSON_PATH}: {e}")
    
    key = "sessionStatus" if session else "status"
    data[key] = status

    try:
        # Atomic write
        fd, temp_path = tempfile.mkstemp(dir=os.path.dirname(SCION_JSON_PATH))
        with os.fdopen(fd, 'w') as f:
            json.dump(data, f, indent=2)
        os.replace(temp_path, SCION_JSON_PATH)
    except Exception as e:
        log_event("ERROR", f"Failed to update {os.path.basename(SCION_JSON_PATH)}: {e}")

def ask_user(message):
    update_status("WAITING_FOR_INPUT", session=True)
    log_event("WAITING_FOR_INPUT", f"Agent requested input: {message}")
    print(f"Agent asked: {message}")

def task_completed(message):
    update_status("COMPLETED", session=True)
    log_event("COMPLETED", f"Agent completed task: {message}")
    print(f"Agent completed: {message}")

def handle_hook():
    try:
        # Check if stdin has data
        if sys.stdin.isatty():
            return
        input_data = json.load(sys.stdin)
    except Exception:
        # Non-JSON input or empty, skip
        return

    event = input_data.get("hook_event_name")
    
    state = "IDLE"
    log_msg = f"Event: {event}"

    if event == "SessionStart":
        state = "STARTING"
        log_msg = f"Session started (source: {input_data.get('source')})"
    
    # User Prompt / Thinking Start
    elif event == "BeforeAgent" or event == "UserPromptSubmit":
        state = "THINKING"
        prompt = input_data.get("prompt", "")
        log_msg = f"User prompt: {prompt[:100]}..." if prompt else "Planning turn"

        if prompt:
            prompt_path = os.path.join(HOME, "prompt.md")
            # Only write if it doesn't exist or is empty
            if not os.path.exists(prompt_path) or os.path.getsize(prompt_path) == 0:
                try:
                    with open(prompt_path, "w") as f:
                        f.write(prompt)
                except Exception as e:
                    log_event("ERROR", f"Failed to save prompt to {prompt_path}: {e}")

    elif event == "BeforeModel":
        state = "THINKING"
        log_msg = "LLM call started"
    
    elif event == "AfterModel":
        state = "IDLE"
        log_msg = "LLM call completed"

    # Tool Execution
    elif event == "BeforeTool" or event == "PreToolUse":
        tool_name = input_data.get("tool_name")
        state = f"EXECUTING ({tool_name})"
        log_msg = f"Running tool: {tool_name}"
    
    elif event == "AfterTool" or event == "PostToolUse":
        state = "IDLE"
        tool_name = input_data.get("tool_name")
        log_msg = f"Tool {tool_name} completed"

    elif event == "Notification":
        state = "WAITING_FOR_INPUT"
        log_msg = f"Notification: {input_data.get('message')}"

    # Turn completion
    elif event == "AfterAgent" or event == "Stop" or event == "SubagentStop":
        state = "IDLE"
        log_msg = "Agent turn completed"
    
    elif event == "SessionEnd":
        state = "EXITED"
        log_msg = f"Session ended (reason: {input_data.get('reason')})"

    update_status(state)
    log_event(state, log_msg)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        handle_hook()
    else:
        command = sys.argv[1]

        if command == "ask_user":
            message = " ".join(sys.argv[2:]) if len(sys.argv) > 2 else "Input requested"
            ask_user(message)
        elif command == "task_completed":
            message = " ".join(sys.argv[2:]) if len(sys.argv) > 2 else "Task completed"
            task_completed(message)
        else:
            print(f"Unknown command: {command}")
            sys.exit(1)
