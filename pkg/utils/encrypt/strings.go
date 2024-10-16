package encrypt

func CensorString(src string, outLength int) string {
	runs := []rune(src)
	out := make([]rune, 0, len(runs))
	if outLength < 2 {
		for i := 0; i < outLength; i++ {
			out = append(out, '*')
		}
		return string(out)
	}
	if len(runs) == 0 {
		for i := 0; i < outLength; i++ {
			out = append(out, '*')
		}
	} else if len(runs) == 1 {
		out = append(out, runs[0])
		for i := 0; i < outLength-2; i++ {
			out = append(out, '*')
		}
		out = append(out, runs[0])
	} else {
		out = append(out, runs[0])
		for i := 0; i < outLength-2; i++ {
			out = append(out, '*')
		}
		out = append(out, runs[len(runs)-1])
	}
	return string(out)
}
