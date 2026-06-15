package dice

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

const (
	ToolName        = "roll_dice"
	ToolDescription = "Roll one or more dice and return the results. Call this whenever the user asks to roll dice or wants a random dice outcome — do not invent dice results yourself."
)

func Roll(sides, count int) (string, error) {
	if sides < 2 || count < 1 || count > 100 {
		return "", fmt.Errorf("invalid dice: sides must be >= 2 and count between 1 and 100")
	}

	rolls := make([]string, count)
	total := 0
	for i := range count {
		roll := rand.IntN(sides) + 1
		total += roll
		rolls[i] = strconv.Itoa(roll)
	}
	if count == 1 {
		return fmt.Sprintf("Rolled a d%d: %s", sides, rolls[0]), nil
	}

	return fmt.Sprintf("Rolled %dd%d: %s (total %d)", count, sides, strings.Join(rolls, ", "), total), nil
}
