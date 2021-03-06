package eulumies

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	keywordRegex      = regexp.MustCompile(`^\[(_*\w*)\]\s+(.*)$`)
	keywordExtraRegex = regexp.MustCompile(`^\s+(.*)$`)
	tiltRegex         = regexp.MustCompile(`^TILT\s*=\s*(.*)$`)
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
	insideBlock   bool
	lastKeyword   string
	strictParsing bool
}

// NewIES reads the given input file and parses it to the IESNA LM-63 data structure.
func NewIES(filepath string, strict bool) (*IES, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ies IES
	ies.strictParsing = strict
	ies.Format = IESFormatUnknown

	scanner := bufio.NewScanner(file)

	// First load all Header fields, 1 to 26
	line, err := validateStringFromLine(scanner, 16, strict)
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
	ies.Keywords = make(map[string]string)
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
		} else if isKeywordExtraLine(line) {
			if err = ies.parseKeywordExtraLine(line); err != nil {
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

	// Parse tilt values.
	if ies.Tilt == IESTiltInclude {
		if ies.TiltLampToLuminaireGeometry, err = getIntFromLine(line); err != nil {
			return nil, err
		}
		line, err = ies.fetchValidLineFromFile(scanner)
		if err != nil {
			return nil, err
		}
		if ies.TiltAnglesAndFactors, err = getIntFromLine(line); err != nil {
			return nil, err
		}

		if words, err := getWordListFromInput(scanner, ies.TiltAnglesAndFactors, false); err != nil {
			return nil, err
		} else {
			if ies.TiltAngles, err = convertStringSliceToFloat(words); err != nil {
				return nil, err
			}
		}
		if words, err := getWordListFromInput(scanner, ies.TiltAnglesAndFactors, false); err != nil {
			return nil, err
		} else {
			if ies.TiltMultiplierFactors, err = convertStringSliceToFloat(words); err != nil {
				return nil, err
			}
		}

	}

	// Parse line 10.
	if words, err := getWordListFromInput(scanner, 10, false); err != nil {
		return nil, err
	} else {
		if ies.NumberLamps, err = strconv.Atoi(words[0]); err != nil {
			return nil, err
		}
		if ies.LumensPerLamp, err = strconv.ParseFloat(words[1], 64); err != nil {
			return nil, err
		}
		if ies.CandelaMultiplier, err = strconv.ParseFloat(words[2], 64); err != nil {
			return nil, err
		}
		if ies.NumberVerticalAngles, err = strconv.Atoi(words[3]); err != nil {
			return nil, err
		}
		if ies.NumberHorizontalAngles, err = strconv.Atoi(words[4]); err != nil {
			return nil, err
		}
		if ies.PhotometricType, err = strconv.Atoi(words[5]); err != nil {
			return nil, err
		}
		if ies.UnitsType, err = strconv.Atoi(words[6]); err != nil {
			return nil, err
		}
		if ies.LuminaireWidth, err = strconv.ParseFloat(words[7], 64); err != nil {
			return nil, err
		}
		if ies.LuminaireLength, err = strconv.ParseFloat(words[8], 64); err != nil {
			return nil, err
		}
		if ies.LuminaireHeight, err = strconv.ParseFloat(words[9], 64); err != nil {
			return nil, err
		}
	}

	// Parse line 11.
	if words, err := getWordListFromInput(scanner, 3, false); err != nil {
		return nil, err
	} else {
		if ies.BallastFactor, err = strconv.ParseFloat(words[1], 64); err != nil {
			return nil, err
		}
		if ies.FutureUse, err = strconv.ParseFloat(words[1], 64); err != nil {
			return nil, err
		}
		if ies.InputWatts, err = strconv.ParseFloat(words[2], 64); err != nil {
			return nil, err
		}
	}

	// Parse vertical angles.
	if words, err := getWordListFromInput(scanner, ies.NumberVerticalAngles, false); err != nil {
		return nil, err
	} else {
		if ies.VerticalAngles, err = convertStringSliceToFloat(words); err != nil {
			return nil, err
		}
	}

	// Parse horizontal angles.
	if words, err := getWordListFromInput(scanner, ies.NumberHorizontalAngles, false); err != nil {
		return nil, err
	} else {
		if ies.HorizontalAngles, err = convertStringSliceToFloat(words); err != nil {
			return nil, err
		}
	}

	// Parse candela values.
	if words, err := getWordListFromInput(scanner, ies.NumberVerticalAngles*ies.NumberHorizontalAngles, true); err != nil {
		return nil, err
	} else {
		if candelaValues, err := convertStringSliceToFloat(words); err != nil {
			return nil, err
		} else {
			c := 0
			ies.CandelaValues = make([][]float64, ies.NumberHorizontalAngles)
			for i := 0; i < ies.NumberHorizontalAngles; i++ {
				ies.CandelaValues[i] = make([]float64, ies.NumberVerticalAngles)
				for j := 0; j < ies.NumberVerticalAngles; j++ {
					ies.CandelaValues[i][j] = candelaValues[c]
					c++
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &ies, nil
}

// Export writes the IESNA LM-63 instance to a file.
func (i *IES) Export(filepath string) error {
	if ok, msg := i.Validate(true); !ok {
		return errors.New(msg)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	lineLength := i.maxKeywordLineLength()

	// Format
	if _, err = file.WriteString(i.convertFormatToString() + "\r\n"); err != nil {
		return err
	}

	// Keywords
	for keyword, value := range i.Keywords {
		var cleanKeywordLines []string
		var splitValue = strings.Split(strings.Replace(value, "\r\n", "\n", -1), "\n")
		maxLineLength := lineLength - len(keyword) - 3 // -3: [ ] and space
		for _, val := range splitValue {
			val = strings.TrimSpace(val)
			if len(val) > maxLineLength {
				chunkSize := maxLineLength

				for j := 0; j < len(val); j += chunkSize {
					end := j + chunkSize

					if end > len(val) {
						end = len(val)
					}

					cleanKeywordLines = append(cleanKeywordLines, strings.TrimSpace(val[j:end]))

				}
			} else {

				cleanKeywordLines = append(cleanKeywordLines, strings.TrimSpace(val))
			}

			// recalculate maxLineLength for next lines depending on the format
			if i.Format == IESFormatLM_63_2002 {
				maxLineLength = lineLength - 7 // [MORE] and space
			} else {
				maxLineLength = lineLength - 1 // space in front
			}
		}

		if len(cleanKeywordLines) == 0 {
			return fmt.Errorf("failed to split keyword %s into line", keyword)
		}

		// Write first line
		if _, err = file.WriteString("[" + keyword + "] " + cleanKeywordLines[0] + "\r\n"); err != nil {
			return err
		}
		if len(cleanKeywordLines) > 1 {
			for l := 1; l < len(cleanKeywordLines); l++ {
				if i.Format == IESFormatLM_63_2002 {
					if _, err = file.WriteString("[MORE] " + cleanKeywordLines[l] + "\r\n"); err != nil {
						return err
					}
				} else {
					if _, err = file.WriteString(" " + cleanKeywordLines[l] + "\r\n"); err != nil {
						return err
					}
				}
			}
		}
	}

	// Tilt Information
	if _, err = file.WriteString("TILT=" + string(i.Tilt) + "\r\n"); err != nil {
		return err
	}

	// Tilt Data
	lineLength = i.maxDataLineLength()
	if i.Tilt == IESTiltInclude {
		if _, err = file.WriteString(strconv.Itoa(i.TiltLampToLuminaireGeometry) + "\r\n"); err != nil {
			return err
		}
		if _, err = file.WriteString(strconv.Itoa(i.TiltAnglesAndFactors) + "\r\n"); err != nil {
			return err
		}
		angleLines := convertFloatSliceToStringSlice(lineLength, i.TiltAngles)
		for _, line := range angleLines {
			if _, err = file.WriteString(line + "\r\n"); err != nil {
				return err
			}
		}
		multiplierLines := convertFloatSliceToStringSlice(lineLength, i.TiltMultiplierFactors)
		for _, line := range multiplierLines {
			if _, err = file.WriteString(line + "\r\n"); err != nil {
				return err
			}
		}
	}

	// Line 10
	lines := convertValuesToStringSlice(lineLength, i.NumberLamps, i.LumensPerLamp, i.CandelaMultiplier,
		i.NumberVerticalAngles, i.NumberHorizontalAngles, i.PhotometricType, i.UnitsType, i.LuminaireWidth,
		i.LuminaireLength, i.LuminaireHeight)
	for _, line := range lines {
		if _, err = file.WriteString(line + "\r\n"); err != nil {
			return err
		}
	}

	// Line 10
	lines = convertValuesToStringSlice(lineLength, i.BallastFactor, i.FutureUse, i.InputWatts)
	for _, line := range lines {
		if _, err = file.WriteString(line + "\r\n"); err != nil {
			return err
		}
	}

	// Vertival angles
	lines = convertFloatSliceToStringSlice(lineLength, i.VerticalAngles)
	for _, line := range lines {
		if _, err = file.WriteString(line + "\r\n"); err != nil {
			return err
		}
	}

	// Horizontal angles
	lines = convertFloatSliceToStringSlice(lineLength, i.HorizontalAngles)
	for _, line := range lines {
		if _, err = file.WriteString(line + "\r\n"); err != nil {
			return err
		}
	}

	// Candela values
	for _, vertAngles := range i.CandelaValues {
		lines = convertFloatSliceToStringSlice(lineLength, vertAngles)
		for _, line := range lines {
			if _, err = file.WriteString(line + "\r\n"); err != nil {
				return err
			}
		}
	}

	if err = file.Sync(); err != nil {
		return err
	}

	return nil
}

// Upgrade sets the format version of the IESNA LM-63 instance to a IESFormatLM_63_2002. It also fixes the required keywords.
func (i *IES) Upgrade() error {
	if ok, msg := i.Validate(true); !ok {
		return errors.New(msg)
	}

	i.Format = IESFormatLM_63_2002

	if !i.ContainsRequiredKeywords() {
		if _, ok := i.Keywords["TEST"]; !ok {
			i.Keywords["TEST"] = "unknown"
		}
		if _, ok := i.Keywords["TESTLAB"]; !ok {
			i.Keywords["TESTLAB"] = "unknown"
		}
		if _, ok := i.Keywords["ISSUEDATE"]; !ok {
			i.Keywords["ISSUEDATE"] = "unknown"
		}
		if _, ok := i.Keywords["MANUFAC"]; !ok {
			i.Keywords["MANUFAC"] = "unknown"
		}
	}

	// Convert not allowed keywords to custom keywords
	for keyword, value := range i.Keywords {
		if !i.isKeywordAllowed(keyword) {
			delete(i.Keywords, keyword)
			if keyword == "DATE" {
				i.Keywords["ISSUEDATE"] = value
			} else {
				i.Keywords["_"+keyword] = value
			}
		}
	}

	return nil
}

// Validate the IESNA LM-63 Data structure
func (i *IES) Validate(strict bool) (bool, string) {
	if strict {
		// TODO: length checks on all fields
	}

	if !i.ContainsRequiredKeywords() {
		return false, "required keywords not present"
	}

	if i.NumberVerticalAngles != len(i.VerticalAngles) {
		return false, "VerticalAngles length mismatch"
	}

	if i.NumberHorizontalAngles != len(i.HorizontalAngles) {
		return false, "HorizontalAngles length mismatch"
	}

	if i.NumberHorizontalAngles != len(i.CandelaValues) {
		return false, "CandelaValues horizontal length mismatch"
	}

	for _, c := range i.CandelaValues {
		if i.NumberVerticalAngles != len(c) {
			return false, "CandelaValues vertical length mismatch"
		}
	}

	return true, ""
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

func (i *IES) convertFormatToString() string {
	switch i.Format {
	case IESFormatLM_63_1986:
		return ""
	case IESFormatLM_63_1991:
		return "IESNA91"
	case IESFormatLM_63_1995:
		return "IESNA:LM-63-1995"
	case IESFormatLM_63_2002:
		return "IESNA:LM-63-2002"
	default:
		return ""
	}
}

func (i *IES) maxKeywordLineLength() int {
	newLineLength := 2 // \r\n
	switch i.Format {
	case IESFormatLM_63_1986:
		return 82 - newLineLength
	case IESFormatLM_63_1991:
		return 82 - newLineLength
	case IESFormatLM_63_1995:
		return 82 - newLineLength
	case IESFormatLM_63_2002:
		return 256 - newLineLength
	default:
		return 0
	}
}

func (i *IES) maxDataLineLength() int {
	newLineLength := 2 // \r\n
	switch i.Format {
	case IESFormatLM_63_1986:
		return 132 - newLineLength
	case IESFormatLM_63_1991:
		return 132 - newLineLength
	case IESFormatLM_63_1995:
		return 132 - newLineLength
	case IESFormatLM_63_2002:
		return 256 - newLineLength
	default:
		return 0
	}
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

func isKeywordExtraLine(line string) bool {
	// TODO: is this allowed in every standard?
	return keywordExtraRegex.MatchString(line)
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

func (i *IES) parseKeywordExtraLine(line string) error {
	matches := keywordExtraRegex.FindStringSubmatch(line)
	value := matches[1]

	if len(i.Keywords) == 0 || i.lastKeyword == "" {
		return fmt.Errorf("extra keyword line occured before any other keyword")
	}

	i.Keywords[i.lastKeyword] += "\n" + value

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
	lineLength := i.maxDataLineLength()

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		} else {
			return "", errors.New("unexpected EOF")
		}
	}

	if len(scanner.Text()) > lineLength && i.strictParsing {
		return "", errors.New("line exceeds maximum allowed length: " + scanner.Text())
	}

	return scanner.Text(), nil
}

func getIntFromLine(line string) (int, error) {
	cleanLine := strings.TrimSpace(line)
	// also replace spaces and underscores
	cleanLine = strings.ReplaceAll(cleanLine, " ", "")
	cleanLine = strings.ReplaceAll(cleanLine, "_", "")

	if len(cleanLine) == 0 {
		return -1, errors.New("line contains no integer")
	}

	value, err := strconv.Atoi(cleanLine)

	return value, err
}

func convertStringSliceToFloat(input []string) ([]float64, error) {
	list := make([]float64, len(input))
	for i, str := range input {
		if flt, err := strconv.ParseFloat(str, 64); err != nil {
			return nil, err
		} else {
			list[i] = flt
		}
	}

	return list, nil
}

func getWordListFromInput(scanner *bufio.Scanner, size int, lastScan bool) ([]string, error) {
	list := make([]string, size)
	processed := 0
	for processed < size {
		wordScanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(scanner.Text())))
		wordScanner.Split(bufio.ScanWords)
		for wordScanner.Scan() {
			list[processed] = strings.TrimSpace(wordScanner.Text())
			processed++
		}

		if processed < size || !lastScan {
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return nil, err
				} else {
					return nil, errors.New("unexpected EOF")
				}
			}
		}
	}

	return list, nil
}

func convertFloatSliceToStringSlice(lineLength int, input []float64) []string {
	var lines []string

	currentLine := ""
	sep := ""
	for _, flt := range input {
		fltStr := strconv.FormatFloat(flt, 'f', 2, 64)
		if len(currentLine)+len(fltStr)+1 > lineLength {
			lines = append(lines, currentLine)
			currentLine = ""
		}
		currentLine += sep + fltStr
		sep = " "
	}
	lines = append(lines, currentLine)

	return lines
}

func convertValuesToStringSlice(lineLength int, input ...interface{}) []string {
	var lines []string

	currentLine := ""
	sep := ""
	for _, val := range input {
		valStr := fmt.Sprint(val)
		if len(currentLine)+len(valStr)+1 > lineLength {
			lines = append(lines, currentLine)
			currentLine = ""
		}
		currentLine += sep + valStr
		sep = " "
	}
	lines = append(lines, currentLine)

	return lines
}
