package reporting

type ConversionCounts struct{ Lead, Contact, Opportunity, Won, Order float64 }

func ConversionResult(c ConversionCounts) ReportResult {
	s := map[string]*float64{"lead": &c.Lead, "contact": &c.Contact, "opportunity": &c.Opportunity, "won": &c.Won, "order": &c.Order}
	return ReportResult{Summary: s, Dimensions: []Dimension{{Key: "lead", Label: "lead", Value: c.Lead}, {Key: "contact", Label: "contact", Value: c.Contact}, {Key: "opportunity", Label: "opportunity", Value: c.Opportunity}, {Key: "won", Label: "won", Value: c.Won}, {Key: "order", Label: "order", Value: c.Order}}}
}
