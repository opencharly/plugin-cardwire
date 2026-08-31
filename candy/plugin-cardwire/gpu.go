package cardwire

import (
	"errors"
	"fmt"
	"os/exec"
)

// gpu.go owns `cardwire gpu <id> block|unblock` — the enable/disable surface. It
// forks /usr/bin/cardwire gpu <id> --block/--unblock, then VERIFIES via
// /usr/bin/cardwire list --json that the GPU's blocked flag actually flipped (the
// daemon enforces mode-dependent semantics; verification is the honest contract, not
// the CLI's success string). Not-installed → "cardwire UNAVAILABLE" exit 1 (this IS
// the mutating surface — refusal is correct). Default-GPU refusals and daemon errors
// pass through with the cardwire prose + exit 1.

func gpuToggle(id uint32, act string) (string, error) {
	flag := "--block"
	verb := "BLOCKED"
	expect := true
	if act == "unblock" {
		flag = "--unblock"
		verb = "UNBLOCKED"
		expect = false
	}

	// 1. The mutating call.
	if _, err := runCardwire("gpu", fmt.Sprint(id), flag); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Sprintf("cardwire gpu %d %s: cardwire UNAVAILABLE (daemon not installed/running)\n", id, act), fmt.Errorf("cardwire gpu %d %s: cardwire UNAVAILABLE (daemon not installed/running)", id, act)
		}
		return fmt.Sprintf("cardwire gpu %d %s: %v\n", id, act, err), fmt.Errorf("cardwire gpu %d %s: %v", id, act, err)
	}

	// 2. Verify the flip via cardwire list --json.
	raw, err := runCardwire("list", "--json")
	if err != nil {
		return fmt.Sprintf("cardwire gpu %d %s: VERIFY FAILED (cardwire list --json: %v)\n", id, act, err), fmt.Errorf("cardwire gpu %d %s: VERIFY FAILED: %v", id, act, err)
	}
	gpus, err := parseCardwireList(raw)
	if err != nil {
		return fmt.Sprintf("cardwire gpu %d %s: VERIFY FAILED (parse: %v)\n", id, act, err), fmt.Errorf("cardwire gpu %d %s: VERIFY FAILED: %v", id, act, err)
	}
	for _, g := range gpus {
		if g.ID == id {
			if g.Blocked != expect {
				return fmt.Sprintf("cardwire gpu %d %s: VERIFY FAILED (blocked=%v, expected %v)\n", id, act, g.Blocked, expect), fmt.Errorf("cardwire gpu %d %s: VERIFY FAILED (blocked=%v, expected %v)", id, act, g.Blocked, expect)
			}
			return fmt.Sprintf("GPU %d %s (verified: blocked=%v)\n", id, verb, g.Blocked), nil
		}
	}
	return fmt.Sprintf("cardwire gpu %d %s: VERIFY FAILED (GPU id %d not in the cardwire list)\n", id, act, id), fmt.Errorf("cardwire gpu %d %s: VERIFY FAILED (GPU id %d not in the cardwire list)", id, act, id)
}

// managerStatus passes through cardwire manager status (read-only; missing cardwire →
// graceful N/A, exit 0 — consistent with list).
func managerStatus() (string, error) {
	out, err := runCardwire("manager", "status")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "cardwire manager status: cardwire UNAVAILABLE (daemon not installed/running)\n", nil
		}
		return "cardwire manager status: " + err.Error() + "\n", err
	}
	return out, nil
}
