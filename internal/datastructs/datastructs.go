package datastructs

import (
	"encoding/xml"
	"time"
)

type CurrencyRates struct {
	Name       string
	Code       string
	Proportion float64
	Date       time.Time
}

type CurrencyList struct {
	XMLName  xml.Name `xml:"CcyTbl"`
	Currency []struct {
		Code string `xml:"Ccy"`
		Name []struct {
			Text string `xml:",chardata"`
			Lang string `xml:"lang,attr"`
		} `xml:"CcyNm"`
	} `xml:"CcyNtry"`
}

type FxRates struct {
	XMLName xml.Name `xml:"FxRates"`
	FxRate  []struct {
		Date    string `xml:"Dt"`
		Curency []struct {
			Code   string `xml:"Ccy"`
			Amount string `xml:"Amt"`
		} `xml:"CcyAmt"`
	} `xml:"FxRate"`
}
