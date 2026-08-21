package execenv

// RenderDesignDeliverySectionForTest exposes the delivered-design brief
// section so the cross-boundary test can assert what an implementing agent is
// actually told.
func RenderDesignDeliverySectionForTest(raw string) string {
	return renderDesignDeliverySection(raw)
}
