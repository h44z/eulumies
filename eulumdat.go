package eulumies

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Reference: http://www.helios32.com/Eulumdat.htm
// Reference: https://docs.agi32.com/PhotometricToolbox/Content/Open_Tool/eulumdat_file_format.htm

// Eulumdat data structure
type Eulumdat struct {
	/* 01 */ CompanyIdentification string // 78 char - Company identification/data bank/version/format identification max.
	/* 02 */ TypeIndicator int // 1  int  - Type indicator I_typ (1 - point source with symmetry about the vertical axis; 2 - linear luminaire; 3 - point source with any other symmetry) [See Note 1]
	/* 03 */ SymmetryIndicator int // 1  int  - Symmetry indicator I_sym (0 - no symmetry; 1 - symmetry about the vertical axis; 2- symmetry to plane C0-C180; 3- symmetry to plane C90-C270; 4- symmetry to plane C0-C180 and to plane C90-C270)
	/* 04 */ NumberMcCPlanes int // 2  int  - Number M_c of C-planes between 0 and 360 degrees (usually 24 for interior, 36 for road lighting luminaires)
	/* 05 */ DistanceDcCPlanes float64 // 5  dbl  - Angular-Distance Dc between C-planes (D_c = 0 for non-equidistantly available C-planes)
	/* 06 */ NumberNgIntensitiesCPlane int // 2  int  - Number N_g of luminous intensities in each C-plane (usually 19 or 37)
	/* 07 */ DistanceDgCPlane float64 // 5  dbl  - Angular-Distance D_g between luminous intensities per C-plane (D_g = 0 for non-equidistantly available luminous intensities in C-planes)
	/* 08 */ MeasurementReportNumber string // 78 char - Measurement report number
	/* 09 */ LuminaireName string // 78 char - Luminaire name
	/* 10 */ LuminaireNumber string // 78 char - Luminaire number
	/* 11 */ FileName string // 8  char - File name (DOS)
	/* 12 */ DateUser string // 78 char - Date / User
	/* 13 */ LengthDiameter float64 // 4  dbl  - Length / diameter of luminaire (mm)
	/* 14 */ WidthLuminaire float64 // 4  dbl  - Width of luminaire b (mm) (b = 0 for circular luminaire)
	/* 15 */ HeightLuminaire float64 // 4  dbl  - Height of luminaire (mm)
	/* 16 */ LengthDiameterLuminousArea float64 // 4  dbl  - Length / diameter of luminous area (mm)
	/* 17 */ WidthLuminousArea float64 // 4  dbl  - Width of luminous area b1 (mm) (b1 = 0 for circular luminous area of luminaire)
	/* 18 */ HeightLuminousAreaC0 float64 // 4  dbl  - Height of luminous area C0-plane (mm)
	/* 19 */ HeightLuminousAreaC90 float64 // 4  dbl  - Height of luminous area C90-plane (mm)
	/* 20 */ HeightLuminousAreaC180 float64 // 4  dbl  - Height of luminous area C180-plane (mm)
	/* 21 */ HeightLuminousAreaC270 float64 // 4  dbl  - Height of luminous area C270-plane (mm)
	/* 22 */ DownwardFluxFractionPhiu float64 // 4  dbl  - Downward flux fraction DFF Phi_u (%)
	/* 23 */ LightOutputRatioLuminaire float64 // 4  dbl  - Light output ratio luminaire LORL, luminair efficiency (%)
	/* 24 */ IntensityConversionFactor float64 // 6  dbl  - Conversion factor for luminous intensities (depending on measurement)
	/* 25 */ MeasurementTiltLuminaire float64 // 6  dbl  - Tilt of luminaire during measurement (road lighting luminaires)
	/* 26 */ NumberStandardSetLamps int // 4  int  - Number n of standard sets of lamps (optional, also extendable on company-specific basis)

	/* 26a */
	NumberLamps []int // n * 4   - Number of lamps
	/* 26b */ TypeLamps []string // n * 24  - Type of lamps
	/* 26c */ TotalLuminousFluxLamps []float64 // n * 12  - Total luminous flux of lamps (lumens)
	/* 26d */ ColorTemperature []string // n * 16  - Color appearance / color temperature of lamps
	/* 26e */ ColorRenderingIndexCRI []string // n * 6   - Color rendering group / color rendering index
	/* 26f */ BallastWatts []float64 // n * 8   - Wattage including ballast (watts)

	/* 27 */
	DirectRatios [10]float64 //  10 * 7   - Direct ratios DR for room indices k = 0.6 ... 5 (for determination of luminaire numbers according to utilization factor method)
	/* 28 */ AnglesC []float64 //  M_c * 6  - Angles C (beginning with 0 degrees)
	/* 29 */ AnglesG []float64 //  N_g * 6  - Angles G (beginning with 0 degrees)
	/* 30 */ LuminousIntensityDistributionRaw []float64 // (M_c2-M_c1+1) * N_g * 6 -  Luminous intensity distribution (candela / 1000 lumens)
	/* 30~ */ LuminousIntensityDistribution [][]float64 // same as raw, but already divided into planes
	/* 30 Hints:
	 *
	 * I_sym    M_c1         M_c2
	 * 0        1            M_c
	 * 1        1            1
	 * 2        1            M_c/2+1
	 * 3        3*M_c/4+1    M_c1 + M_c/2
	 * 4        1            M_c/4+1
	 */

	// Internal variables, used for calculation only
	mc1 int
	mc2 int
	mc  int
}

// EulumdatAssembly represents one data-set for rows 26.a-f
type EulumdatAssembly struct {
	Current             float64 // either the current or -1 if the default currents of the modules have been used
	NumberOfLamps       int
	TypeOfLamps         string
	TotalLuminousFlux   float64
	Power               float64
	ColorTemperature    string
	ColorRenderingIndex string
}

// NewEulumdat reads the given input file and parses it to the Eulumdat data structure.
func NewEulumdat(in io.Reader, strict bool) (Eulumdat, error) {
	var eulumdat Eulumdat
	var err error
	scanner := bufio.NewScanner(in)

	// First load all Header fields, 1 to 26
	if eulumdat.CompanyIdentification, err = validateStringFromLine(scanner, 78, strict); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.TypeIndicator, err = validateIntFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.SymmetryIndicator, err = validateIntFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.NumberMcCPlanes, err = validateIntFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.DistanceDcCPlanes, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.NumberNgIntensitiesCPlane, err = validateIntFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.DistanceDgCPlane, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.MeasurementReportNumber, err = validateStringFromLine(scanner, 78, strict); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.LuminaireName, err = validateStringFromLine(scanner, 78, strict); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.LuminaireNumber, err = validateStringFromLine(scanner, 78, strict); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.FileName, err = validateStringFromLine(scanner, 8, strict); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.DateUser, err = validateStringFromLine(scanner, 78, strict); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.LengthDiameter, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.WidthLuminaire, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.HeightLuminaire, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.LengthDiameterLuminousArea, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.WidthLuminousArea, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.HeightLuminousAreaC0, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.HeightLuminousAreaC90, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.HeightLuminousAreaC180, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.HeightLuminousAreaC270, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.DownwardFluxFractionPhiu, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.LightOutputRatioLuminaire, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.IntensityConversionFactor, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.MeasurementTiltLuminaire, err = validateFloatFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}
	if eulumdat.NumberStandardSetLamps, err = validateIntFromLine(scanner); err != nil {
		return Eulumdat{}, err
	}

	// Now load measurement data 26a to 26f
	eulumdat.NumberLamps = make([]int, eulumdat.NumberStandardSetLamps)
	eulumdat.TypeLamps = make([]string, eulumdat.NumberStandardSetLamps)
	eulumdat.TotalLuminousFluxLamps = make([]float64, eulumdat.NumberStandardSetLamps)
	eulumdat.ColorTemperature = make([]string, eulumdat.NumberStandardSetLamps)
	eulumdat.ColorRenderingIndexCRI = make([]string, eulumdat.NumberStandardSetLamps)
	eulumdat.BallastWatts = make([]float64, eulumdat.NumberStandardSetLamps)
	for i := 0; i < eulumdat.NumberStandardSetLamps; i++ {
		if eulumdat.NumberLamps[i], err = validateIntFromLine(scanner); err != nil {
			return Eulumdat{}, err
		}
		if eulumdat.TypeLamps[i], err = validateStringFromLine(scanner, 24, strict); err != nil {
			return Eulumdat{}, err
		}
		if eulumdat.TotalLuminousFluxLamps[i], err = validateFloatFromLine(scanner); err != nil {
			return Eulumdat{}, err
		}
		if eulumdat.ColorTemperature[i], err = validateStringFromLine(scanner, 16, strict); err != nil {
			return Eulumdat{}, err
		}
		if eulumdat.ColorRenderingIndexCRI[i], err = validateStringFromLine(scanner, 6, strict); err != nil {
			return Eulumdat{}, err
		}
		if eulumdat.BallastWatts[i], err = validateFloatFromLine(scanner); err != nil {
			return Eulumdat{}, err
		}
	}

	// Now load the 10 ratios from field 27
	for i := 0; i < 10; i++ {
		if eulumdat.DirectRatios[i], err = validateFloatFromLine(scanner); err != nil {
			return Eulumdat{}, err
		}
	}

	// Load all C angles, field 28 and all G angles, field 29
	eulumdat.AnglesC = make([]float64, eulumdat.NumberMcCPlanes)
	for i := 0; i < eulumdat.NumberMcCPlanes; i++ {
		if eulumdat.AnglesC[i], err = validateFloatFromLine(scanner); err != nil {
			return Eulumdat{}, err
		}
	}
	eulumdat.AnglesG = make([]float64, eulumdat.NumberNgIntensitiesCPlane)
	for i := 0; i < eulumdat.NumberNgIntensitiesCPlane; i++ {
		if eulumdat.AnglesG[i], err = validateFloatFromLine(scanner); err != nil {
			return Eulumdat{}, err
		}
	}

	// Calculate M_c1 and M_c2 to load the luminous intensity distribution data from field 30
	eulumdat.calcMc1andMc2()
	dataLength := (eulumdat.mc2 - eulumdat.mc1 + 1) * eulumdat.NumberNgIntensitiesCPlane
	eulumdat.LuminousIntensityDistributionRaw = make([]float64, dataLength)
	for i := 0; i < dataLength; i++ {
		// All luminous intensities
		if eulumdat.LuminousIntensityDistributionRaw[i], err = validateFloatFromLine(scanner); err != nil {
			return Eulumdat{}, err
		}
	}

	// Split luminous intensities into planes
	// Details can be found in QLumEdit Source (eulumdat.cpp, line 234)
	if err = eulumdat.CalcLuminousIntensityDistributionFromRaw(); err != nil {
		return Eulumdat{}, err
	}

	if err := scanner.Err(); err != nil {
		return Eulumdat{}, err
	}

	return eulumdat, nil
}

// CopyEulumdat creates a deep copy of the given Eulumdat instance.
func CopyEulumdat(source Eulumdat) (Eulumdat, error) {
	copyObject := source

	// Deep copy reference fields
	copyObject.NumberLamps = make([]int, len(source.NumberLamps))
	copy(copyObject.NumberLamps, source.NumberLamps)
	copyObject.TypeLamps = make([]string, len(source.TypeLamps))
	copy(copyObject.TypeLamps, source.TypeLamps)
	copyObject.TotalLuminousFluxLamps = make([]float64, len(source.TotalLuminousFluxLamps))
	copy(copyObject.TotalLuminousFluxLamps, source.TotalLuminousFluxLamps)
	copyObject.ColorTemperature = make([]string, len(source.ColorTemperature))
	copy(copyObject.ColorTemperature, source.ColorTemperature)
	copyObject.ColorRenderingIndexCRI = make([]string, len(source.ColorRenderingIndexCRI))
	copy(copyObject.ColorRenderingIndexCRI, source.ColorRenderingIndexCRI)
	copyObject.BallastWatts = make([]float64, len(source.BallastWatts))
	copy(copyObject.BallastWatts, source.BallastWatts)

	copyObject.AnglesC = make([]float64, len(source.AnglesC))
	copy(copyObject.AnglesC, source.AnglesC)
	copyObject.AnglesG = make([]float64, len(source.AnglesG))
	copy(copyObject.AnglesG, source.AnglesG)
	copyObject.LuminousIntensityDistributionRaw = make([]float64, len(source.LuminousIntensityDistributionRaw))
	copy(copyObject.LuminousIntensityDistributionRaw, source.LuminousIntensityDistributionRaw)
	copyObject.LuminousIntensityDistribution = make([][]float64, len(source.LuminousIntensityDistribution))
	copy(copyObject.LuminousIntensityDistribution, source.LuminousIntensityDistribution)
	for i := range source.LuminousIntensityDistribution {
		copyObject.LuminousIntensityDistribution[i] = make([]float64, len(source.LuminousIntensityDistribution[i]))
		copy(copyObject.LuminousIntensityDistribution[i], source.LuminousIntensityDistribution[i])
	}

	return copyObject, nil
}

// Export writes the Eulumdat instance to a file.
func (e Eulumdat) Export(out io.StringWriter) error {
	if ok, msg := e.Validate(false); !ok {
		return errors.New(msg)
	}

	var err error
	if _, err = out.WriteString(e.CompanyIdentification + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(strconv.Itoa(e.TypeIndicator) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(strconv.Itoa(e.SymmetryIndicator) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(strconv.Itoa(e.NumberMcCPlanes) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.DistanceDcCPlanes) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(strconv.Itoa(e.NumberNgIntensitiesCPlane) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.DistanceDgCPlane) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(e.MeasurementReportNumber + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(e.LuminaireName + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(e.LuminaireNumber + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(e.FileName + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(e.DateUser + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.LengthDiameter) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.WidthLuminaire) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.HeightLuminaire) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.LengthDiameterLuminousArea) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.WidthLuminousArea) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.HeightLuminousAreaC0) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.HeightLuminousAreaC90) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.HeightLuminousAreaC180) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.HeightLuminousAreaC270) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.DownwardFluxFractionPhiu) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.LightOutputRatioLuminaire) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.IntensityConversionFactor) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(fmt.Sprintf("%f", e.MeasurementTiltLuminaire) + "\r\n"); err != nil {
		return err
	}
	if _, err = out.WriteString(strconv.Itoa(e.NumberStandardSetLamps) + "\r\n"); err != nil {
		return err
	}

	// 26a - 26f
	for i := 0; i < e.NumberStandardSetLamps; i++ {
		if _, err = out.WriteString(strconv.Itoa(e.NumberLamps[i]) + "\r\n"); err != nil {
			return err
		}
		if _, err = out.WriteString(e.TypeLamps[i] + "\r\n"); err != nil {
			return err
		}
		if _, err = out.WriteString(fmt.Sprintf("%f", e.TotalLuminousFluxLamps[i]) + "\r\n"); err != nil {
			return err
		}
		if _, err = out.WriteString(e.ColorTemperature[i] + "\r\n"); err != nil {
			return err
		}
		if _, err = out.WriteString(e.ColorRenderingIndexCRI[i] + "\r\n"); err != nil {
			return err
		}
		if _, err = out.WriteString(fmt.Sprintf("%f", e.BallastWatts[i]) + "\r\n"); err != nil {
			return err
		}
	}

	// 27
	for i := 0; i < 10; i++ {
		if _, err = out.WriteString(fmt.Sprintf("%f", e.DirectRatios[i]) + "\r\n"); err != nil {
			return err
		}
	}

	// 28
	for i := 0; i < e.NumberMcCPlanes; i++ {
		if _, err = out.WriteString(fmt.Sprintf("%f", e.AnglesC[i]) + "\r\n"); err != nil {
			return err
		}
	}

	// 29
	for i := 0; i < e.NumberNgIntensitiesCPlane; i++ {
		if _, err = out.WriteString(fmt.Sprintf("%f", e.AnglesG[i]) + "\r\n"); err != nil {
			return err
		}
	}

	// 30
	e.calcMc1andMc2()
	dataLength := (e.mc2 - e.mc1 + 1) * e.NumberNgIntensitiesCPlane
	for i := 0; i < dataLength; i++ {
		if _, err = out.WriteString(fmt.Sprintf("%f", e.LuminousIntensityDistributionRaw[i]) + "\r\n"); err != nil {
			return err
		}
	}

	return nil
}

// Calculate the value of Mc1 and Mc2 based on the symmetry indicator.
//      I_sym    M_c1         M_c2
//      0        1            M_c
//      1        1            1
//      2        1            M_c/2+1
//      3        3*M_c/4+1    M_c1 + M_c/2
//      4        1            M_c/4+1
func (e *Eulumdat) calcMc1andMc2() {
	switch e.SymmetryIndicator {
	case 0:
		e.mc1 = 1
		e.mc2 = e.NumberMcCPlanes
	case 1:
		e.mc1 = 1
		e.mc2 = 1
	case 2:
		e.mc1 = 1
		e.mc2 = e.NumberMcCPlanes/2 + 1
	case 3:
		e.mc1 = 3*e.NumberMcCPlanes/4 + 1
		e.mc2 = e.mc1 + e.NumberMcCPlanes/2
	case 4:
		e.mc1 = 1
		e.mc2 = e.NumberMcCPlanes/4 + 1
	}
}

// Calculate the value of Mc based on the symmetry indicator.
// This values is used to split the raw value into planes.
func (e *Eulumdat) calcMc() {
	switch e.SymmetryIndicator {
	case 0:
		e.mc = e.NumberMcCPlanes
	case 1:
		e.mc = 1
	case 2:
		fallthrough
	case 3:
		e.mc = e.NumberMcCPlanes/2 + 1
	case 4:
		e.mc = e.NumberMcCPlanes/4 + 1
	}
}

// CalcLuminousIntensityDistributionFromRaw splits luminous intensities into planes
func (e *Eulumdat) CalcLuminousIntensityDistributionFromRaw() error {
	e.calcMc()
	e.LuminousIntensityDistribution = make([][]float64, e.mc)
	for i := 0; i < e.mc; i++ { // Mc is the number C-Planes
		start := i * e.NumberNgIntensitiesCPlane
		end := start + e.NumberNgIntensitiesCPlane

		length := end - start
		e.LuminousIntensityDistribution[i] = make([]float64, length)
		for j := 0; j < length; j++ {
			e.LuminousIntensityDistribution[i][j] = e.LuminousIntensityDistributionRaw[start+j]
		}
	}

	return nil
}

// Validate the EULUMDAT Data structure
func (e Eulumdat) Validate(strict bool) (bool, string) {
	if strict {
		// TODO: length checks on all fields
	}

	if e.NumberStandardSetLamps != len(e.NumberLamps) {
		return false, "NumberLamps length mismatch"
	}
	if e.NumberStandardSetLamps != len(e.TypeLamps) {
		return false, "TypeLamps length mismatch"
	}
	if e.NumberStandardSetLamps != len(e.TotalLuminousFluxLamps) {
		return false, "TotalLuminousFluxLamps length mismatch"
	}
	if e.NumberStandardSetLamps != len(e.ColorTemperature) {
		return false, "ColorTemperature length mismatch"
	}
	if e.NumberStandardSetLamps != len(e.ColorRenderingIndexCRI) {
		return false, "ColorRenderingIndexCRI length mismatch"
	}
	if e.NumberStandardSetLamps != len(e.BallastWatts) {
		return false, "BallastWatts length mismatch"
	}
	if e.NumberMcCPlanes != len(e.AnglesC) {
		return false, "AnglesC length mismatch"
	}
	if e.NumberNgIntensitiesCPlane != len(e.AnglesG) {
		return false, "AnglesG length mismatch"
	}

	e.calcMc1andMc2()
	dataLength := (e.mc2 - e.mc1 + 1) * e.NumberNgIntensitiesCPlane
	if dataLength != len(e.LuminousIntensityDistributionRaw) {
		return false, "LuminousIntensityDistributionRaw length mismatch"
	}

	return true, ""
}

// GetMaximumLuminousIntensity returns the maximum luminous intensity for the given C-Plane
func (e Eulumdat) GetMaximumLuminousIntensity(planeIndex int) float64 {
	max := 0.0
	planeIntensities := e.LuminousIntensityDistribution[planeIndex]
	for _, intensity := range planeIntensities {
		max = math.Max(max, intensity)
	}

	return max
}

// GetOverallMaximumLuminousIntensity returns the maximum luminous intensity of all C-Planes
func (e Eulumdat) GetOverallMaximumLuminousIntensity() float64 {
	max := 0.0
	for _, intensity := range e.LuminousIntensityDistributionRaw {
		max = math.Max(max, intensity)
	}

	return max
}

// GetFwhm returns the full width at half maximum angle.
func (e Eulumdat) GetFwhm(planeIndex int) float64 {
	if planeIndex == -1 || planeIndex >= e.mc {
		return -1 // plane does not exist
	}

	// only makes sense if luminaire field is symmetric
	if e.SymmetryIndicator != 1 && e.SymmetryIndicator != 4 {
		return -1
	}

	maxIntensity := e.GetMaximumLuminousIntensity(planeIndex)
	targetIntensity := maxIntensity / 2

	// find the closest angle to halfMaxI
	minDiff := math.MaxFloat64 // init as large as possible as we want to find the minimum

	angle := -1.0
	planeIntensities := e.LuminousIntensityDistribution[planeIndex]
	for intensityIndex, intensity := range planeIntensities {
		diff := math.Abs(intensity - targetIntensity)
		if diff < minDiff && e.AnglesG[intensityIndex] <= 90 { // <= 90, assume that ivmax is located between 0 and 90 °
			minDiff = diff
			angle = e.AnglesG[intensityIndex]
		}
	}

	if angle < 0 {
		return -1
	}

	return angle * 2
}

// GetFwtm returns the full width at 1/10 maximum angle.
func (e Eulumdat) GetFwtm(planeIndex int) float64 {
	if planeIndex == -1 || planeIndex >= e.mc {
		return -1 // plane does not exist
	}

	// only makes sense if luminaire field is symmetric
	if e.SymmetryIndicator != 1 && e.SymmetryIndicator != 4 {
		return -1
	}

	maxIntensity := e.GetMaximumLuminousIntensity(planeIndex)
	targetIntensity := maxIntensity / 10

	// find the closest angle to halfMaxI
	minDiff := math.MaxFloat64 // init as large as possible as we want to find the minimum

	angle := -1.0
	planeIntensities := e.LuminousIntensityDistribution[planeIndex]
	for intensityIndex, intensity := range planeIntensities {
		diff := math.Abs(intensity - targetIntensity)
		if diff < minDiff && e.AnglesG[intensityIndex] <= 90 { // <= 90, assume that ivmax is located between 0 and 90 °
			minDiff = diff
			angle = e.AnglesG[intensityIndex]
		}
	}

	if angle < 0 {
		return -1
	}

	return angle * 2
}

// GetCPlaneIndex returns the internal index of the C-Plane for the given angle.
// If no such plane was found, -1 is returned.
func (e Eulumdat) GetCPlaneIndex(angle float64) int {
	for i, planeAngle := range e.AnglesC {
		if planeAngle == angle {
			return i
		}
	}

	return -1
}

func validateStringFromLine(scanner *bufio.Scanner, maxLength int, strict bool) (string, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		} else {
			return "", errors.New("unexpected EOF")
		}
	}
	cleanLine := strings.TrimSpace(scanner.Text())
	if len(cleanLine) > maxLength && strict {
		return "", errors.New("line exceeds maximum allowed length: " + cleanLine)
	} else if len(cleanLine) > maxLength && !strict {
		//logrus.Tracef("[EULUM] line exceeds maximum allowed length: %d > %d, %s", len(cleanLine), maxLength, cleanLine)
	}
	return cleanLine, nil
}

func validateIntFromLine(scanner *bufio.Scanner) (int, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return -1, err
		} else {
			return -1, errors.New("unexpected EOF")
		}
	}

	cleanLine := strings.TrimSpace(scanner.Text())
	// also replace spaces and underscores
	cleanLine = strings.ReplaceAll(cleanLine, " ", "")
	cleanLine = strings.ReplaceAll(cleanLine, "_", "")

	if len(cleanLine) == 0 {
		return -1, errors.New("line contains no integer")
	}

	value, err := strconv.Atoi(cleanLine)

	return value, err
}

func validateFloatFromLine(scanner *bufio.Scanner) (float64, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return -1, err
		} else {
			return -1, errors.New("unexpected EOF")
		}
	}

	cleanLine := strings.TrimSpace(scanner.Text())
	// replace all commas if present with dots
	cleanLine = strings.ReplaceAll(cleanLine, ",", ".")
	// also replace spaces and underscores
	cleanLine = strings.ReplaceAll(cleanLine, " ", "")
	cleanLine = strings.ReplaceAll(cleanLine, "_", "")

	if len(cleanLine) == 0 {
		return -1, errors.New("line contains no float")
	}

	value, err := strconv.ParseFloat(cleanLine, 64)

	return value, err
}

// CalculateEulumdatAssemblies returns an ordered list of assemblies, the assembly with the highest current is the first element.
func CalculateEulumdatAssemblies(luminaireData LuminaireData, luminousPoints float64) ([]EulumdatAssembly, error) {
	assemblies := make([]EulumdatAssembly, len(luminaireData.PossibleCurrents))
	for i, current := range luminaireData.PossibleCurrents {
		assemblies[i] = EulumdatAssembly{
			Current:     float64(current),
			TypeOfLamps: "LED",
		}
		assemblies[i].ColorTemperature = mapColorTempsToString(luminaireData.GetUniqueColorTemperatures(current))
		assemblies[i].ColorRenderingIndex = fmt.Sprintf("%0.0f", luminaireData.GetMinimalCri(current))
		assemblies[i].Power = luminaireData.GetRealTotalPower(current) / luminousPoints
		assemblies[i].TotalLuminousFlux = luminaireData.GetTotalLuminousFlux(current) / luminousPoints
		assemblies[i].NumberOfLamps = luminaireData.GetNumberOfLamps(luminousPoints)
	}

	sort.Slice(assemblies, func(i, j int) bool {
		return assemblies[i].Current > assemblies[j].Current
	})

	return assemblies, nil
}

func ApplyEulumdatAssemblies(assemblies []EulumdatAssembly, eulumdat *Eulumdat) {
	eulumdat.NumberLamps = make([]int, len(assemblies))
	eulumdat.TypeLamps = make([]string, len(assemblies))
	eulumdat.ColorTemperature = make([]string, len(assemblies))
	eulumdat.BallastWatts = make([]float64, len(assemblies))
	eulumdat.TotalLuminousFluxLamps = make([]float64, len(assemblies))
	eulumdat.ColorRenderingIndexCRI = make([]string, len(assemblies))
	eulumdat.NumberStandardSetLamps = len(assemblies)
	for i := range assemblies {
		eulumdat.NumberLamps[i] = assemblies[i].NumberOfLamps
		eulumdat.TypeLamps[i] = assemblies[i].TypeOfLamps
		eulumdat.ColorTemperature[i] = assemblies[i].ColorTemperature
		eulumdat.BallastWatts[i] = assemblies[i].Power
		eulumdat.TotalLuminousFluxLamps[i] = assemblies[i].TotalLuminousFlux
		eulumdat.ColorRenderingIndexCRI[i] = assemblies[i].ColorRenderingIndex
	}
}
