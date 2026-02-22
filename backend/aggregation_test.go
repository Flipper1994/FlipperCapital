package main

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestAggregateOHLCV_DayBoundary(t *testing.T) {
	// Simulate PRG-like 60m bars: 7 bars per day (9:30-15:30 ET), 3 days
	loc, _ := time.LoadLocation("America/New_York")

	makeBars := func(dates []string) []OHLCV {
		var bars []OHLCV
		hours := []int{9, 10, 11, 12, 13, 14, 15} // :30 each
		price := 30.0
		for _, dateStr := range dates {
			dt, _ := time.ParseInLocation("2006-01-02", dateStr, loc)
			for _, h := range hours {
				ts := time.Date(dt.Year(), dt.Month(), dt.Day(), h, 30, 0, 0, loc)
				bars = append(bars, OHLCV{
					Time:   ts.Unix(),
					Open:   price,
					High:   price + 0.5,
					Low:    price - 0.5,
					Close:  price + 0.1,
					Volume: 1000,
				})
				price += 0.1
			}
		}
		return bars
	}

	bars := makeBars([]string{"2026-02-16", "2026-02-17", "2026-02-18"})

	// Test 4h aggregation
	agg4h := aggregateOHLCV(bars, 4)
	t.Logf("=== 4h aggregation: %d input bars → %d output bars ===", len(bars), len(agg4h))

	for i, bar := range agg4h {
		ts := time.Unix(bar.Time, 0).In(loc)
		t.Logf("  Bar %2d: %s  O=%.2f C=%.2f", i, ts.Format("2006-01-02 15:04 MST"), bar.Open, bar.Close)
	}

	// Verify NO bar crosses day boundaries
	for i, bar := range agg4h {
		startDay := time.Unix(bar.Time, 0).In(loc).Format("2006-01-02")
		// Find the last source bar in this group
		var lastSourceTime int64
		for _, b := range bars {
			if b.Time >= bar.Time {
				if i+1 < len(agg4h) && b.Time >= agg4h[i+1].Time {
					break
				}
				lastSourceTime = b.Time
			}
		}
		endDay := time.Unix(lastSourceTime, 0).In(loc).Format("2006-01-02")
		if startDay != endDay {
			t.Errorf("Bar %d crosses day boundary: %s → %s", i, startDay, endDay)
		}
	}

	// Verify each day produces exactly 2 bars (4h + 3h remainder)
	dayBars := make(map[string]int)
	for _, bar := range agg4h {
		day := time.Unix(bar.Time, 0).In(loc).Format("2006-01-02")
		dayBars[day]++
	}
	for day, count := range dayBars {
		if count != 2 {
			t.Errorf("Day %s: expected 2 bars (4h + 3h), got %d", day, count)
		}
	}

	// Verify 4h bars start at 9:30 (market open) each day
	for _, bar := range agg4h {
		ts := time.Unix(bar.Time, 0).In(loc)
		if ts.Minute() != 30 {
			t.Errorf("Bar at %s: expected :30 minute alignment", ts.Format("15:04"))
		}
	}

	// Test 2h aggregation
	agg2h := aggregateOHLCV(bars, 2)
	t.Logf("\n=== 2h aggregation: %d input bars → %d output bars ===", len(bars), len(agg2h))

	for i, bar := range agg2h {
		ts := time.Unix(bar.Time, 0).In(loc)
		t.Logf("  Bar %2d: %s  O=%.2f C=%.2f", i, ts.Format("2006-01-02 15:04 MST"), bar.Open, bar.Close)
	}

	// Verify NO 2h bar crosses day boundaries
	for i, bar := range agg2h {
		startDay := time.Unix(bar.Time, 0).In(loc).Format("2006-01-02")
		var lastSourceTime int64
		for _, b := range bars {
			if b.Time >= bar.Time {
				if i+1 < len(agg2h) && b.Time >= agg2h[i+1].Time {
					break
				}
				lastSourceTime = b.Time
			}
		}
		endDay := time.Unix(lastSourceTime, 0).In(loc).Format("2006-01-02")
		if startDay != endDay {
			t.Errorf("2h Bar %d crosses day boundary: %s → %s", i, startDay, endDay)
		}
	}

	// Each day should have 4 bars for 2h (2h + 2h + 2h + 1h remainder)
	dayBars2h := make(map[string]int)
	for _, bar := range agg2h {
		day := time.Unix(bar.Time, 0).In(loc).Format("2006-01-02")
		dayBars2h[day]++
	}
	for day, count := range dayBars2h {
		if count != 4 {
			t.Errorf("2h Day %s: expected 4 bars (2h+2h+2h+1h), got %d", day, count)
		}
	}

	_ = math.Abs // keep import
}

func TestAggregateOHLCV_RealYahooTimestamps(t *testing.T) {
	// Real PRG timestamps from Yahoo (60m, ET, Feb 2026)
	loc, _ := time.LoadLocation("America/New_York")
	mkTS := func(dateStr string, hour, min int) int64 {
		dt, _ := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %02d:%02d", dateStr, hour, min), loc)
		return dt.Unix()
	}

	bars := []OHLCV{
		// Feb 18
		{Time: mkTS("2026-02-18", 9, 30), Open: 35.59, High: 35.60, Low: 34.50, Close: 34.57, Volume: 100},
		{Time: mkTS("2026-02-18", 10, 30), Open: 34.57, High: 35.20, Low: 34.50, Close: 35.14, Volume: 100},
		{Time: mkTS("2026-02-18", 11, 30), Open: 35.14, High: 35.80, Low: 35.10, Close: 35.77, Volume: 100},
		{Time: mkTS("2026-02-18", 12, 30), Open: 35.77, High: 35.80, Low: 35.40, Close: 35.48, Volume: 100},
		{Time: mkTS("2026-02-18", 13, 30), Open: 35.51, High: 35.55, Low: 35.40, Close: 35.45, Volume: 100},
		{Time: mkTS("2026-02-18", 14, 30), Open: 35.43, High: 35.45, Low: 35.30, Close: 35.38, Volume: 100},
		{Time: mkTS("2026-02-18", 15, 30), Open: 35.38, High: 36.20, Low: 35.35, Close: 36.13, Volume: 100},
		// Feb 19 — big gap up
		{Time: mkTS("2026-02-19", 9, 30), Open: 36.90, High: 41.00, Low: 36.80, Close: 40.81, Volume: 500},
		{Time: mkTS("2026-02-19", 10, 30), Open: 40.95, High: 41.00, Low: 39.50, Close: 39.78, Volume: 300},
		{Time: mkTS("2026-02-19", 11, 30), Open: 39.87, High: 40.10, Low: 39.70, Close: 40.02, Volume: 200},
		{Time: mkTS("2026-02-19", 12, 30), Open: 40.01, High: 40.10, Low: 39.70, Close: 39.78, Volume: 200},
		{Time: mkTS("2026-02-19", 13, 30), Open: 39.78, High: 40.10, Low: 39.70, Close: 40.08, Volume: 150},
		{Time: mkTS("2026-02-19", 14, 30), Open: 40.06, High: 40.50, Low: 40.00, Close: 40.48, Volume: 150},
		{Time: mkTS("2026-02-19", 15, 30), Open: 40.46, High: 40.50, Low: 40.30, Close: 40.43, Volume: 100},
	}

	agg4h := aggregateOHLCV(bars, 4)

	t.Log("=== PRG 4h (fixed) ===")
	for i, bar := range agg4h {
		ts := time.Unix(bar.Time, 0).In(loc)
		t.Logf("  Bar %d: %s  O=%.2f H=%.2f L=%.2f C=%.2f",
			i, ts.Format("2006-01-02 15:04 MST"), bar.Open, bar.High, bar.Low, bar.Close)
	}

	// Feb 18 should produce 2 bars: 9:30 (4 bars) + 13:30 (3 bars)
	// Feb 19 should produce 2 bars: 9:30 (4 bars) + 13:30 (3 bars)
	if len(agg4h) != 4 {
		t.Fatalf("Expected 4 bars (2 per day × 2 days), got %d", len(agg4h))
	}

	// Bar 0: Feb 18 09:30-12:30 (4 bars)
	bar0 := time.Unix(agg4h[0].Time, 0).In(loc)
	if bar0.Hour() != 9 || bar0.Minute() != 30 {
		t.Errorf("Bar 0: expected 09:30, got %s", bar0.Format("15:04"))
	}
	if agg4h[0].Open != 35.59 || agg4h[0].Close != 35.48 {
		t.Errorf("Bar 0: expected O=35.59 C=35.48, got O=%.2f C=%.2f", agg4h[0].Open, agg4h[0].Close)
	}

	// Bar 1: Feb 18 13:30-15:30 (3 bars) — NOT crossing into Feb 19!
	bar1 := time.Unix(agg4h[1].Time, 0).In(loc)
	if bar1.Format("2006-01-02") != "2026-02-18" {
		t.Errorf("Bar 1: expected Feb 18, got %s", bar1.Format("2006-01-02"))
	}
	if bar1.Hour() != 13 || bar1.Minute() != 30 {
		t.Errorf("Bar 1: expected 13:30, got %s", bar1.Format("15:04"))
	}
	if agg4h[1].Close != 36.13 {
		t.Errorf("Bar 1: expected C=36.13 (day close), got C=%.2f", agg4h[1].Close)
	}

	// Bar 2: Feb 19 09:30-12:30 (4 bars) — gap up should be HERE
	bar2 := time.Unix(agg4h[2].Time, 0).In(loc)
	if bar2.Format("2006-01-02") != "2026-02-19" {
		t.Errorf("Bar 2: expected Feb 19, got %s", bar2.Format("2006-01-02"))
	}
	if agg4h[2].Open != 36.90 {
		t.Errorf("Bar 2: expected O=36.90 (gap up open), got O=%.2f", agg4h[2].Open)
	}

	// Bar 3: Feb 19 13:30-15:30 (3 bars)
	bar3 := time.Unix(agg4h[3].Time, 0).In(loc)
	if bar3.Format("2006-01-02") != "2026-02-19" {
		t.Errorf("Bar 3: expected Feb 19, got %s", bar3.Format("2006-01-02"))
	}
}

func TestAggregateOHLCV_FactorOne(t *testing.T) {
	bars := []OHLCV{{Time: 1000, Open: 10, Close: 11}, {Time: 2000, Open: 12, Close: 13}}
	result := aggregateOHLCV(bars, 1)
	if len(result) != len(bars) {
		t.Errorf("factor=1: expected %d bars, got %d", len(bars), len(result))
	}
}

func TestAggregateOHLCV_Empty(t *testing.T) {
	result := aggregateOHLCV(nil, 4)
	if len(result) != 0 {
		t.Errorf("empty input: expected 0 bars, got %d", len(result))
	}
}
