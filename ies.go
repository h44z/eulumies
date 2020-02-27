package eulumies

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
)

// Reference: https://knowledge.autodesk.com/support/3ds-max/learn-explore/caas/CloudHelp/cloudhelp/2015/ENU/3DSMax/files/GUID-EA0E3DE0-275C-42F7-83EC-429A37B2D501-htm.html
// Reference: https://docs.agi32.com/PhotometricToolbox/Content/Open_Tool/iesna_lm-63_format.htm

type IESFormat string

const (
	IESFormatUnknown    IESFormat = "UNKNOWN"
	IESFormatLM_63_1986 IESFormat = "LM-63-1986"
	IESFormatLM_63_1991 IESFormat = "LM-63-1991"
	IESFormatLM_63_1995 IESFormat = "LM-63-1995"
	IESFormatLM_63_2002 IESFormat = "LM-63-2002"
)

type IESTilt string

const (
	IESTiltInclude IESTilt = "INCLUDE" // The lamp output varies as a function of the luminaire tilt angle.
	IESTiltFile    IESTilt = "FILE"    // The lamp output varies as a function of the luminaire tilt angle. Requires a filename.
	IESTiltNone    IESTilt = "NONE"    // The lamp output (presumably) does not vary as a function of the luminaire tilt angle.
)

var (
	keywordRegex = regexp.MustCompile(`^(\[\w*\])\s+(.*)$`)
	tiltRegex    = regexp.MustCompile(`^TILT\s*=\s*(.*)$`)
)

// IESNA LM-63 data structure
type IES struct {
	Format                      IESFormat         // first line - IES file format and version definition
	Keywords                    map[string]string // Keyword MORE or OTHER can occur multiple times. User defined keywords start with _.
	Tilt                        IESTilt
	TiltLampToLuminaireGeometry int       // only if tilt == INCLUDE, indicates the orientation of the lamp within the luminaire (can be 1, 2 or 3)
	TiltAnglesAndFactors        int       // only if tilt == INCLUDE, indicates the total number of lamp tilt angles and their corresponding candela multiplying factors
	TiltAngles                  []float64 // only if tilt == INCLUDE
	TiltMultiplierFactors       []float64 // only if tilt == INCLUDE
	NumberLamps                 int
	LumensPerLamp               float64
	CandelaMultiplier           float64
	NumberVerticalAngles        int
	NumberHorizontalAngles      int
	PhotometricType             int // 1, 2 or 3
	UnitsType                   int // 1 = feet, 2 = meters
	LuminaireWidth              float64
	LuminaireLength             float64
	LuminaireHeight             float64
	BallastFactor               float64
	FutureUse                   float64
	InputWatts                  float64
	VerticalAngles              []float64
	HorizontalAngles            []float64
	CandelaValues               [][]float64 // candela values for all vertical angles per	horizontal angle

	// internal parser values
	insideBlock bool
	lastKeyword string
}

func NewIES(filepath string) (*IES, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ies IES
	ies.Format = IESFormatUnknown

	scanner := bufio.NewScanner(file)

	// First load all Header fields, 1 to 26
	line, err := validateStringFromLine(scanner, 16, true)
	if err != nil {
		return nil, err
	}
	if err = ies.parseFormatVersion(line); err != nil {
		return nil, err
	}

	line, err = ies.fetchValidLineFromFile(scanner)
	if err != nil {
		return nil, err
	}

	// Parse keywords and tilt information.
	tiltReached := false
	for !tiltReached {
		if isKeywordLine(line) {
			if err = ies.parseKeywordLine(line); err != nil {
				return nil, err
			}
		} else if isTiltLine(line) {
			if !ies.ContainsRequiredKeywords() {
				return nil, fmt.Errorf("required keywords are missing")
			}
			tiltReached = true

			if err = ies.parseTiltLine(line); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("expected keyword or tilt line, not %s", line)
		}

		line, err = ies.fetchValidLineFromFile(scanner)
		if err != nil {
			return nil, err
		}
	}

	// Parse main luminaire values.
	// TODO: implement

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &ies, nil
}

func (i *IES) parseFormatVersion(line string) error {
	switch line {
	case "IESNA91":
		i.Format = IESFormatLM_63_1991
	case "IESNA:LM-63-1995":
		i.Format = IESFormatLM_63_1995
	case "IESNA:LM-63-2002":
		i.Format = IESFormatLM_63_2002
	default:
		return fmt.Errorf("invalid ies format %s", line) // Might be IESFormatLM_63_1986, but this is not supported
	}

	return nil
}

func (i *IES) isKeywordAllowed(keyword string) bool {
	if keyword == "" {
		return false
	}

	if len(keyword) > 18 { // Max allowed keyword length by standard.
		return false
	}

	if i.Format == IESFormatUnknown || i.Format == "" {
		return true // Cannot check if no format is set.
	}

	if keyword[0] == '_' {
		return true // Allow private/custom keywords
	}

	switch i.Format {
	case IESFormatLM_63_1986:
		return true
	case IESFormatLM_63_1991:
		return keywordAllowedByIesna91(keyword)
	case IESFormatLM_63_1995:
		return keywordAllowedByIesna95(keyword)
	case IESFormatLM_63_2002:
		return keywordAllowedByIesna02(keyword)
	}

	return true
}

func keywordAllowedByIesna02(keyword string) bool {
	if keyword == "TEST" ||
		keyword == "TESTLAB" ||
		keyword == "TESTDATE" ||
		keyword == "NEARFIELD" ||
		keyword == "MANUFAC" ||
		keyword == "LUMCAT" ||
		keyword == "LUMINAIRE" ||
		keyword == "LAMPCAT" ||
		keyword == "LAMP" ||
		keyword == "BALLAST" ||
		keyword == "BALLASTCAT" ||
		keyword == "MAINTCAT" ||
		keyword == "DISTRIBUTION" ||
		keyword == "FLASHAREA" ||
		keyword == "COLORCONSTANT" ||
		keyword == "LAMPPOSITION" ||
		keyword == "ISSUEDATE" ||
		keyword == "OTHER" ||
		keyword == "SEARCH" ||
		keyword == "MORE" {
		return true
	}
	return false
}

func keywordAllowedByIesna95(keyword string) bool {
	if keyword == "TEST" ||
		keyword == "DATE" ||
		keyword == "NEARFIELD" ||
		keyword == "MANUFAC" ||
		keyword == "LUMCAT" ||
		keyword == "LUMINAIRE" ||
		keyword == "LAMPCAT" ||
		keyword == "LAMP" ||
		keyword == "BALLAST" ||
		keyword == "BALLASTCAT" ||
		keyword == "MAINTCAT" ||
		keyword == "DISTRIBUTION" ||
		keyword == "FLASHAREA" ||
		keyword == "COLORCONSTANT" ||
		keyword == "OTHER" ||
		keyword == "SEARCH" ||
		keyword == "MORE" ||
		keyword == "BLOCK" ||
		keyword == "ENDBLOCK" {
		return true
	}
	return false
}

func keywordAllowedByIesna91(keyword string) bool {
	if keyword == "TEST" ||
		keyword == "DATE" ||
		keyword == "MANUFAC" ||
		keyword == "LUMCAT" ||
		keyword == "LUMINAIRE" ||
		keyword == "LAMPCAT" ||
		keyword == "LAMP" ||
		keyword == "BALLAST" ||
		keyword == "BALLASTCAT" ||
		keyword == "MAINTCAT" ||
		keyword == "DISTRIBUTION" ||
		keyword == "FLASHAREA" ||
		keyword == "COLORCONSTANT" ||
		keyword == "MORE" {
		return true
	}
	return false
}

func (i *IES) ContainsRequiredKeywords() bool {
	if i.Format == IESFormatUnknown || i.Format == "" {
		return true // Cannot check if no format is set.
	}

	switch i.Format {
	case IESFormatLM_63_1986:
		return true // This format does not contain any keywords.
	case IESFormatLM_63_1991:
		return checkIesna91RequiredKeywords(i.Keywords)
	case IESFormatLM_63_1995:
		return true // No required keywords.
	case IESFormatLM_63_2002:
		return checkIesna02RequiredKeywords(i.Keywords)
	}

	return true
}

func checkIesna02RequiredKeywords(keywords map[string]string) bool {
	requiredKeywords := [...]string{
		"TEST",
		"TESTLAB",
		"ISSUEDATE",
		"MANUFAC",
	}

	for _, keyword := range requiredKeywords {
		if _, ok := keywords[keyword]; !ok {
			return false
		}
	}

	return true
}

func checkIesna91RequiredKeywords(keywords map[string]string) bool {
	requiredKeywords := [...]string{
		"TEST",
		"MANUFAC",
	}

	for _, keyword := range requiredKeywords {
		if _, ok := keywords[keyword]; !ok {
			return false
		}
	}

	return true
}

func isKeywordLine(line string) bool {
	return keywordRegex.MatchString(line)
}

func isTiltLine(line string) bool {
	return tiltRegex.MatchString(line)
}

func (i *IES) parseKeywordLine(line string) error {
	matches := keywordRegex.FindStringSubmatch(line)
	keyword := matches[1]
	value := matches[2]

	// Check if the specified standard allows this keyword.
	if !i.isKeywordAllowed(keyword) {
		return fmt.Errorf("keyword %s is not allowed for standard %s", keyword, i.Format)
	}

	// Check for BLOCK and ENDBLOCK keywords
	if !i.checkKeywordBlock(keyword) {
		return fmt.Errorf("unexpected block/endblock keyword")
	}

	if keyword == "MORE" {
		if len(i.Keywords) == 0 || i.lastKeyword == "" {
			return fmt.Errorf("keyword MORE occured before any other keyword")
		}

		i.Keywords[i.lastKeyword] += "\n" + value
	} else {
		i.Keywords[keyword] = value
		i.lastKeyword = keyword
	}

	return nil
}

func (i *IES) parseTiltLine(line string) error {
	matches := tiltRegex.FindStringSubmatch(line)
	value := matches[1]

	if value == "INCLUDE" {
		i.Tilt = IESTiltInclude
	} else if value == "NONE" {
		i.Tilt = IESTiltNone
	} else {
		i.Tilt = IESTiltFile
		return fmt.Errorf("TILT specification from file is not supported")
	}

	return nil
}

func (i *IES) checkKeywordBlock(keyword string) bool {
	if keyword == "BLOCK" {
		if i.insideBlock {
			return false // BLOCK keyword inside of block is not expected.
		}
		i.insideBlock = true
	} else if keyword == "ENDBLOCK" {
		if !i.insideBlock {
			return false // ENDBLOCK keyword outside of block is not expected.
		}

		i.insideBlock = false
	}

	return true
}

func (i *IES) fetchValidLineFromFile(scanner *bufio.Scanner) (string, error) {
	lineLength := 256 // TODO: set according to chosen format
	line, err := validateStringFromLine(scanner, lineLength, true)
	if err != nil {
		return "", err
	}
	return line, nil
}
