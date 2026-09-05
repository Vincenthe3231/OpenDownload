package media

import "strings"

// Format is an advertised media representation that can be selected for a
// download. ID must match the identifier shown to the user.
type Format struct {
	ID        string
	Bandwidth int
	Width     int
	Height    int
	Extension string
}

// SelectFormatIndex returns the selected format index. Empty selection means
// best. Exact format IDs are matched before a missing selection is rejected.
func SelectFormatIndex(formats []Format, selection string) (int, bool) {
	if len(formats) == 0 {
		return 0, false
	}

	selection = strings.TrimSpace(selection)
	switch strings.ToLower(selection) {
	case "", SelectionBest:
		return bestFormatIndex(formats), true
	case SelectionWorst:
		return worstFormatIndex(formats), true
	default:
		for index, format := range formats {
			if format.ID == selection {
				return index, true
			}
		}
		return 0, false
	}
}

// SelectFormat returns the selected advertised format.
func SelectFormat(formats []Format, selection string) (Format, bool) {
	index, ok := SelectFormatIndex(formats, selection)
	if !ok {
		return Format{}, false
	}
	return formats[index], true
}

func bestFormatIndex(formats []Format) int {
	selected := 0
	for index := 1; index < len(formats); index++ {
		if formats[index].Bandwidth > formats[selected].Bandwidth {
			selected = index
		}
	}
	return selected
}

func worstFormatIndex(formats []Format) int {
	selected := 0
	for index := 1; index < len(formats); index++ {
		if formats[index].Bandwidth < formats[selected].Bandwidth {
			selected = index
		}
	}
	return selected
}
