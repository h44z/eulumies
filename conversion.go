package eulumies

func ConvertEulumdatToIES(eulumdat *Eulumdat) (*IES, error) {
	ies := &IES{
		Format: IESFormatLM_63_2002,
		Tilt:   IESTiltNone,
	}
	ies.Keywords = make(map[string]string)
	ies.Keywords["TEST"] = eulumdat.MeasurementReportNumber
	ies.Keywords["TESTLAB"] = eulumdat.CompanyIdentification
	ies.Keywords["ISSUEDATE"] = eulumdat.DateUser
	ies.Keywords["MANUFAC"] = eulumdat.CompanyIdentification
	ies.Keywords["LUMINAIRE"] = eulumdat.LuminaireName
	ies.Keywords["LUMCAT"] = eulumdat.LuminaireNumber
	ies.Keywords["LAMP"] = eulumdat.TypeLamps[0]
	ies.Keywords["OTHER"] = "converted using eulumies: " + eulumdat.FileName

	ies.NumberLamps = eulumdat.NumberLamps[0]
	ies.LumensPerLamp = eulumdat.TotalLuminousFluxLamps[0]
	ies.CandelaMultiplier = 1 // TODO
	ies.NumberVerticalAngles = len(eulumdat.AnglesG)
	ies.NumberHorizontalAngles = 1 // TODO
	ies.PhotometricType = 1        // TODO
	ies.UnitsType = 2
	ies.LuminaireWidth = eulumdat.WidthLuminaire
	ies.LuminaireLength = eulumdat.LengthDiameter
	ies.LuminaireHeight = eulumdat.HeightLuminaire
	ies.BallastFactor = 1
	ies.FutureUse = 1
	ies.InputWatts = eulumdat.BallastWatts[0]
	ies.VerticalAngles = eulumdat.AnglesG
	ies.HorizontalAngles = []float64{0.0}
	ies.CandelaValues = eulumdat.LuminousIntensityDistribution

	return ies, nil
}

func ConvertIESToEulumdat(ies *IES) (*Eulumdat, error) {
	return nil, nil
}
