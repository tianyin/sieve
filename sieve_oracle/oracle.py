import yaml
import os
from sieve_common.common import *
from sieve_common.default_config import sieve_config
from sieve_oracle.checker_common import *
from sieve_oracle.safety_checker import *
from sieve_oracle.liveness_checker import *


def persistent_history_and_state(test_context: TestContext):
    if sieve_config["generic_event_generation_enabled"]:
        history = generate_history(test_context)
        history_digest = generate_history_digest(test_context)
        dump_json_file(test_context.result_dir, history, "history.json")
        dump_json_file(test_context.result_dir, history_digest, "event.json")
    if sieve_config["generic_state_generation_enabled"]:
        state = generate_state(test_context)
        # ignore_paths = generate_state_mask(resources)
        # we generate state.json at src_dir (log dir)
        dump_json_file(test_context.result_dir, state, "state.json")
        # dump_json_file(test_context.result_dir, ignore_paths, "mask.json")
        # we generate state.json at dest_dir (data dir) if cononicalize_resource=True
        # if canonicalize_resource:
        #     dump_json_file(test_context.oracle_dir, resources, "state.json")
        #     dump_json_file(test_context.oracle_dir, ignore_paths, "mask.json")


def canonicalize_history_and_state(test_context: TestContext):
    assert test_context.mode == sieve_modes.LEARN_TWICE
    if sieve_config["generic_event_generation_enabled"]:
        can_history_digest = canonicalize_history_digest(test_context)
        dump_json_file(test_context.oracle_dir, can_history_digest, "event.json")
    if sieve_config["generic_state_generation_enabled"]:
        can_state = canonicalize_state(test_context)
        dump_json_file(test_context.oracle_dir, can_state, "state.json")
        state_mask = generate_state_mask(can_state)
        dump_json_file(test_context.oracle_dir, state_mask, "mask.json")


def operator_checker(test_context: TestContext):
    operator_log = os.path.join(test_context.result_dir, "streamed-operator.log")
    ret_val = 0
    messages = []
    file = open(operator_log)
    for line in file.readlines():
        if "Observed a panic" in line:
            panic_in_file = line[line.find("Observed a panic") :]
            messages.append(generate_alarm("[OPERATOR-PANIC]", panic_in_file.strip()))
            ret_val += 1
    messages.sort()
    return ret_val, messages


def test_workload_checker(test_context: TestContext):
    workload_log = os.path.join(test_context.result_dir, "workload.log")
    ret_val = 0
    messages = []
    file = open(workload_log)
    for line in file.readlines():
        if line.startswith("error:"):
            ret_val += 1
            messages.append(generate_alarm("[WORKLOAD]", line.strip()))
    messages.sort()
    return ret_val, messages


def print_error_and_debugging_info(ret_val, messages, test_config):
    if ret_val == 0:
        return
    test_config_content = yaml.safe_load(open(test_config))
    report_color = bcolors.FAIL if ret_val > 0 else bcolors.WARNING
    cprint("[RET VAL] {}\n".format(ret_val) + messages, report_color)
    if sieve_config["injection_desc_generation_enabled"]:
        hint = "[DEBUGGING SUGGESTION]\n" + generate_debugging_hint(test_config_content)
        cprint(hint, bcolors.WARNING)


def safety_checker(test_context: TestContext):
    ret_val = 0
    messages = []
    if (
        sieve_config["generic_event_checker_enabled"]
        and test_context.mode != sieve_modes.OBS_GAP
    ):
        write_ret_val, write_messages = compare_history_digests(test_context)
        ret_val += write_ret_val
        messages.extend(write_messages)
    return ret_val, messages


def check(test_context: TestContext):
    ret_val = 0
    messages = []

    validation_ret_val, validation_messages = injection_validation(test_context)
    if validation_ret_val < 0:
        messages.extend(validation_messages)

    if sieve_config["operator_checker_enabled"]:
        panic_ret_val, panic_messages = operator_checker(test_context)
        ret_val += panic_ret_val
        messages.extend(panic_messages)

    if sieve_config["test_workload_checker_enabled"]:
        workload_ret_val, workload_messages = test_workload_checker(test_context)
        ret_val += workload_ret_val
        messages.extend(workload_messages)

    # if sieve_config["generic_event_checker_enabled"]:
    write_ret_val, write_messages = safety_checker(test_context)
    ret_val += write_ret_val
    messages.extend(write_messages)

    if sieve_config["generic_state_checker_enabled"]:
        resource_ret_val, resource_messages = generic_state_checker(test_context)
        ret_val += resource_ret_val
        messages.extend(resource_messages)

    if validation_ret_val < 0:
        ret_val = validation_ret_val

    return ret_val, "\n".join(messages)