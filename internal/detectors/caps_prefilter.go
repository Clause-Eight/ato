package detectors

// capsThenDigit is a cheap prefilter for identifiers that start with exactly n capital letters: it looks for
// such a run at the start of a word with a digit within the next gap+1 characters.
func capsThenDigit(line string, n, gap int) bool {
	for i := 0; i+n < len(line); i++ {
		if i > 0 && isWordByte(line[i-1]) {
			continue
		}
		j := i
		for j < len(line) && line[j] >= 'A' && line[j] <= 'Z' {
			j++
		}
		if j-i == n {
			for k := j; k < len(line) && k <= j+gap; k++ {
				if line[k] >= '0' && line[k] <= '9' {
					return true
				}
			}
		}
		if j > i {
			i = j - 1
		}
	}
	return false
}
