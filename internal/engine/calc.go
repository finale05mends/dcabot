package engine

import (
	"dcabot/internal/models"
	"math"
)

func CalcAvgPrice(totalCost, totalQty float64) float64 {
	if totalQty == 0 {
		return 0
	}
	return totalCost / totalQty
}

func CalcTPPrice(avgPrice, tpPercent float64, side models.OrderSide) float64 {
	factor := tpPercent / 100.0

	if side == models.OrderSideBuy {
		return avgPrice * (1 + factor)
	}
	return avgPrice * (1 - factor)
}

func RoundDown(value, step float64) float64 {
	if step == 0 {
		return value
	}
	return math.Floor(value/step) * step
}

type SafetyOrder struct {
	Price        float64
	Qty          float64
	TotalPercent float64
}

func CalcSafetyOrders(entryPrice float64, soCount int, soStepPercent, soStepMultiplier, soBaseQty, soQtyMultiplier float64, side models.OrderSide) []SafetyOrder {
	var orders []SafetyOrder

	totalPercent := 0.0

	for i := 0; i < soCount; i++ {
		step := soStepPercent * math.Pow(soStepMultiplier, float64(i))
		totalPercent += step
		qty := soBaseQty * math.Pow(soQtyMultiplier, float64(i))
		price := entryPrice

		if side == models.OrderSideBuy {
			price = entryPrice * (1 - totalPercent/100.0)
		} else {
			price = entryPrice * (1 + totalPercent/100.0)
		}

		orders = append(orders, SafetyOrder{
			Price:        price,
			Qty:          qty,
			TotalPercent: totalPercent,
		})
	}
	return orders
}
