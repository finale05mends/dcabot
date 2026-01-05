package engine

import (
	"testing"

	"dcabot/internal/models"
)

func TestRoundDown(t *testing.T) {
	got := RoundDown(10.123, 0.01)
	if !almostEqual(got, 10.12, 1e-9) {
		t.Fatalf("Ожидали 10.12, получили %f.", got)
	}
}

func TestCalcTPPrice(t *testing.T) {
	price := CalcTPPrice(100, 0.5, models.OrderSideBuy)
	if !almostEqual(price, 100.5, 1e-9) {
		t.Fatalf("Ожидали 100.5, получили %f.", price)
	}
	price = CalcTPPrice(100, 0.5, models.OrderSideSell)
	if !almostEqual(price, 99.5, 1e-9) {
		t.Fatalf("Ожидали 99.5, получили %f.", price)
	}
}

func TestCalcSafetyOrders(t *testing.T) {
	ordersBuy := CalcSafetyOrders(100, 3, 1, 1.2, 10, 1.1, models.OrderSideBuy)
	ordersSell := CalcSafetyOrders(100, 3, 1, 1.2, 10, 1.1, models.OrderSideSell)
	if len(ordersBuy) != 3 {
		t.Fatalf("Ожидали 3 ордера, получили %d.", len(ordersBuy))
	}
	if ordersBuy[0].Price > 100 {
		t.Fatalf("Ожидали цену ниже входа(100), получили %f.", ordersBuy[0].Price)
	}
	if ordersSell[0].Price < 100 {
		t.Fatalf("Ожидали цену выше входа(100), получили %f.", ordersBuy[0].Price)
	}

	if ordersBuy[1].Price > ordersBuy[0].Price {
		t.Fatalf("Ожидали, что второй ордер ниже первого.")
	}

	if ordersSell[1].Price < ordersSell[0].Price {
		t.Fatalf("Ожидали, что второй ордер выше первого.")
	}

	if !almostEqual(ordersBuy[0].TotalPercent, 1.0, 1e-12) {
		t.Fatalf("Ожидали 1.0, получили %f.", ordersBuy[0].TotalPercent)
	}
	if !almostEqual(ordersBuy[0].Price, 99.0, 1e-12) {
		t.Fatalf("Ожидали 99.0, получили %f.", ordersBuy[0].Price)
	}
	if !almostEqual(ordersBuy[0].Qty, 10.0, 1e-12) {
		t.Fatalf("Ожидали 10.0, получили %f.", ordersBuy[0].Qty)
	}

	if !almostEqual(ordersBuy[1].TotalPercent, 2.2, 1e-12) {
		t.Fatalf("Ожидали 2.2, получили %f.", ordersBuy[1].TotalPercent)
	}
	if !almostEqual(ordersBuy[1].Price, 97.8, 1e-12) {
		t.Fatalf("Ожидали 97.8, получили %f.", ordersBuy[1].Price)
	}
	if !almostEqual(ordersBuy[1].Qty, 11.0, 1e-12) {
		t.Fatalf("Ожидали 11.0, получили %f.", ordersBuy[1].Qty)
	}

	if !almostEqual(ordersBuy[2].TotalPercent, 3.64, 1e-12) {
		t.Fatalf("Ожидали 3.64, получили %f.", ordersBuy[2].TotalPercent)
	}
	if !almostEqual(ordersBuy[2].Price, 96.36, 1e-12) {
		t.Fatalf("Ожидали 96.36, получили %f.", ordersBuy[2].Price)
	}
	if !almostEqual(ordersBuy[2].Qty, 12.1, 1e-12) {
		t.Fatalf("Ожидали 12.1, получили %f.", ordersBuy[2].Qty)
	}

	if !almostEqual(ordersSell[0].TotalPercent, 1.0, 1e-12) {
		t.Fatalf("Ожидали 1.0, получили %f.", ordersSell[0].TotalPercent)
	}
	if !almostEqual(ordersSell[0].Price, 101.0, 1e-12) {
		t.Fatalf("Ожидали 101.0, получили %f.", ordersSell[0].Price)
	}
	if !almostEqual(ordersSell[0].Qty, 10.0, 1e-12) {
		t.Fatalf("Ожидали 10.0, получили %f.", ordersSell[0].Qty)
	}

	if !almostEqual(ordersSell[1].TotalPercent, 2.2, 1e-12) {
		t.Fatalf("Ожидали 2.2, получили %f.", ordersSell[1].TotalPercent)
	}
	if !almostEqual(ordersSell[1].Price, 102.2, 1e-12) {
		t.Fatalf("Ожидали 102.2, получили %f.", ordersSell[1].Price)
	}
	if !almostEqual(ordersSell[1].Qty, 11.0, 1e-12) {
		t.Fatalf("Ожидали 11.0, получили %f.", ordersSell[1].Qty)
	}

	if !almostEqual(ordersSell[2].TotalPercent, 3.64, 1e-12) {
		t.Fatalf("Ожидали 3.64, получили %f.", ordersSell[2].TotalPercent)
	}
	if !almostEqual(ordersSell[2].Price, 103.64, 1e-12) {
		t.Fatalf("Ожидали 103.64, получили %f.", ordersSell[2].Price)
	}
	if !almostEqual(ordersSell[2].Qty, 12.1, 1e-12) {
		t.Fatalf("Ожидали 12.1, получили %f.", ordersSell[2].Qty)
	}
}

func TestCalcAvgPrice(t *testing.T) {
	avg := CalcAvgPrice(150, 3)
	if !almostEqual(avg, 50, 1e-9) {
		t.Fatalf("Ожидали 50, получили %f.", avg)
	}
	avg = CalcAvgPrice(150, 0.0)
	if !almostEqual(avg, 0, 1e-9) {
		t.Fatalf("Ожидали 0.0, получили %f.", avg)
	}
}

func almostEqual(a, b, eps float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= eps
}
