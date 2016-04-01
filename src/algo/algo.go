package algo

import (
	"strings"
	"unicode"

	"github.com/mjwestcott/fzf/src/util"
)

/*
 * String matching algorithms here do not use strings.ToLower to avoid
 * performance penalty. And they assume pattern runes are given in lowercase
 * letters when caseSensitive is false.
 *
 * In short: They try to do as little work as possible.
 */

func runeAt(runes []rune, index int, max int, forward bool) rune {
	if forward {
		return runes[index]
	}
	return runes[max-index-1]
}

// Result conatins the results of running a match function.
type Result struct {
	Start int
	End   int

	// Every result is assigned a penalty based on the distances of the
	// matching runes from the beginning of its containing word. The basic
	// idea is to assign values to each rune in the input text. Then,
	// add up those values which are matched by the pattern. The only nuance
	// is that consecutive matches have no penalty.
	//
	//     input    "Hello, world! This is a test."
	//     values    12345--12345--1234-12-1-1234-
	//     pattern          wo     th        tes
	//     penalties        10     10        100
	//     total = 3
	//
	// Now an example that should be heavily penalized because many of the
	// matches occur in the middle of words:
	//
	//     input    "/usr/jg/repos/go/src/github.com/junegunn"
	//     values    -123-12-12345-12-123-123456-123-12345678
	//     pattern     s       p   g      git            gunn
	//     penalties   2       3   1      100            5000
	//     total = 12
	//
	// We can then decide how to use that penalty when ranking items. One
	// simple and effective idea is to rank according to matchlen + penalty.
	Penalty int32
}

// FuzzyMatch performs fuzzy-match
func FuzzyMatch(caseSensitive bool, forward bool, runes []rune, pattern []rune) *Result {
	if len(pattern) == 0 {
		return &Result{0, 0, 0}
	}

	// 0. (FIXME) How to find the shortest match?
	//    a_____b__c__abc
	//    ^^^^^^^^^^  ^^^
	// 1. forward scan (abc)
	//   *-----*-----*>
	//   a_____b___abc__
	// 2. reverse scan (cba)
	//   a_____b___abc__
	//            <***
	pidx := 0
	sidx := -1
	eidx := -1

	lenRunes := len(runes)
	lenPattern := len(pattern)

	for index := range runes {
		char := runeAt(runes, index, lenRunes, forward)

		// This is considerably faster than blindly applying strings.ToLower to the
		// whole string
		if !caseSensitive {
			// Partially inlining `unicode.ToLower`. Ugly, but makes a noticeable
			// difference in CPU cost. (Measured on Go 1.4.1. Also note that the Go
			// compiler as of now does not inline non-leaf functions.)
			if char >= 'A' && char <= 'Z' {
				char += 32
			} else if char > unicode.MaxASCII {
				char = unicode.To(unicode.LowerCase, char)
			}
		}
		pchar := runeAt(pattern, pidx, lenPattern, forward)
		if char == pchar {
			if sidx < 0 {
				sidx = index
			}
			if pidx++; pidx == lenPattern {
				eidx = index + 1
				break
			}
		}
	}

	if sidx >= 0 && eidx >= 0 {
		pidx--
		for index := eidx - 1; index >= sidx; index-- {
			char := runeAt(runes, index, lenRunes, forward)
			if !caseSensitive {
				if char >= 'A' && char <= 'Z' {
					char += 32
				} else if char > unicode.MaxASCII {
					char = unicode.To(unicode.LowerCase, char)
				}
			}

			pchar := runeAt(pattern, pidx, lenPattern, forward)
			if char == pchar {
				if pidx--; pidx < 0 {
					sidx = index
					break
				}
			}
		}

		// Calculate the penalty. This can't be done at the same time as the
		// pattern scan above because 'forward' may be false.
		var fromBoundary int32
		var totalPenalty int32
		var consecutive bool
		var pidx int

		// We can think about how to start closer to sidx.
		for index := 0; index < eidx; index++ {
			var penalty int32

			// Calculate current rune penalty.
			char := runes[index]
			if unicode.IsLetter(char) || unicode.IsNumber(char) {
				fromBoundary++
				penalty = fromBoundary
			} else {
				fromBoundary = 0
			}

			// Calculate totalPenalty of the match.
			if index >= sidx {
				if !caseSensitive {
					if char >= 'A' && char <= 'Z' {
						char += 32
					} else if char > unicode.MaxASCII {
						char = unicode.To(unicode.LowerCase, char)
					}
				}
				pchar := pattern[pidx]
				if pchar == char {
					if !consecutive {
						totalPenalty += penalty
					}
					if pidx++; pidx == lenPattern {
						break
					}
					consecutive = true
				} else {
					consecutive = false
				}
			}
		}

		if forward {
			return &Result{sidx, eidx, totalPenalty}
		}
		return &Result{lenRunes - eidx, lenRunes - sidx, totalPenalty}
	}
	return &Result{-1, -1, 0}
}

// ExactMatchNaive is a basic string searching algorithm that handles case
// sensitivity. Although naive, it still performs better than the combination
// of strings.ToLower + strings.Index for typical fzf use cases where input
// strings and patterns are not very long.
//
// We might try to implement better algorithms in the future:
// http://en.wikipedia.org/wiki/String_searching_algorithm
func ExactMatchNaive(caseSensitive bool, forward bool, runes []rune, pattern []rune) *Result {
	// Note: ExactMatchNaive always return a zero penalty.
	if len(pattern) == 0 {
		return &Result{0, 0, 0}
	}

	lenRunes := len(runes)
	lenPattern := len(pattern)

	if lenRunes < lenPattern {
		return &Result{-1, -1, 0}
	}

	pidx := 0
	for index := 0; index < lenRunes; index++ {
		char := runeAt(runes, index, lenRunes, forward)
		if !caseSensitive {
			if char >= 'A' && char <= 'Z' {
				char += 32
			} else if char > unicode.MaxASCII {
				char = unicode.To(unicode.LowerCase, char)
			}
		}
		pchar := runeAt(pattern, pidx, lenPattern, forward)
		if pchar == char {
			pidx++
			if pidx == lenPattern {
				if forward {
					return &Result{index - lenPattern + 1, index + 1, 0}
				}
				return &Result{lenRunes - (index + 1), lenRunes - (index - lenPattern + 1), 0}
			}
		} else {
			index -= pidx
			pidx = 0
		}
	}
	return &Result{-1, -1, 0}
}

// PrefixMatch performs prefix-match
func PrefixMatch(caseSensitive bool, forward bool, runes []rune, pattern []rune) *Result {
	// Note: PrefixMatch always return a zero penalty.
	if len(runes) < len(pattern) {
		return &Result{-1, -1, 0}
	}

	for index, r := range pattern {
		char := runes[index]
		if !caseSensitive {
			char = unicode.ToLower(char)
		}
		if char != r {
			return &Result{-1, -1, 0}
		}
	}
	return &Result{0, len(pattern), 0}
}

// SuffixMatch performs suffix-match
func SuffixMatch(caseSensitive bool, forward bool, input []rune, pattern []rune) *Result {
	// Note: SuffixMatch always return a zero penalty.
	runes := util.TrimRight(input)
	trimmedLen := len(runes)
	diff := trimmedLen - len(pattern)
	if diff < 0 {
		return &Result{-1, -1, 0}
	}

	for index, r := range pattern {
		char := runes[index+diff]

		if !caseSensitive {
			char = unicode.ToLower(char)
		}
		if char != r {
			return &Result{-1, -1, 0}
		}
	}
	return &Result{trimmedLen - len(pattern), trimmedLen, 0}
}

// EqualMatch performs equal-match
func EqualMatch(caseSensitive bool, forward bool, runes []rune, pattern []rune) *Result {
	// Note: EqualMatch always return a zero penalty.
	if len(runes) != len(pattern) {
		return &Result{-1, -1, 0}
	}
	runesStr := string(runes)
	if !caseSensitive {
		runesStr = strings.ToLower(runesStr)
	}
	if runesStr == string(pattern) {
		return &Result{0, len(pattern), 0}
	}
	return &Result{-1, -1, 0}
}
