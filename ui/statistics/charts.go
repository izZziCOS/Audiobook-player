package statistics

import (
	"fmt"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/stats"
	"github.com/izzzicos/audiobook-player/ui/kit"
)

const (
	chartPlotHeight   = 150
	chartHeadroom     = 12
	chartAxisGap      = 6
	chartLabelGap     = 8
	chartMaxBar       = 24
	chartBarGap       = 2
	chartBarRadius    = 4
	chartTooltipInset = 8
	heatmapCell       = 16
	heatmapMinCell    = 8
	heatmapGap        = 3
)

var heatmapLevels = []float64{0, 15 * 60, 30 * 60, 60 * 60}

func chartTicks(maximum float64) []float64 {
	steps := []float64{60, 300, 600, 900, 1800, 3600, 7200, 10800, 18000, 36000, 72000, 180000}

	step := steps[len(steps)-1]
	for _, candidate := range steps {
		if maximum/candidate <= 4 {
			step = candidate
			break
		}
	}

	count := max(int(math.Ceil(maximum/step)), 1)
	ticks := make([]float64, count+1)
	for i := range ticks {
		ticks[i] = float64(i) * step
	}

	return ticks
}

func chartTickLabel(seconds float64) string {
	switch {
	case seconds == 0:
		return "0"
	case math.Mod(seconds, 3600) == 0:
		return fmt.Sprintf("%dh", int(seconds/3600))
	case seconds > 3600:
		return kit.FormatLength(seconds)
	default:
		return fmt.Sprintf("%dm", int(seconds/60))
	}
}

func heatmapLevel(seconds float64) int {
	level := 0
	for i, threshold := range heatmapLevels {
		if seconds > threshold {
			level = i + 1
		}
	}

	return level
}

func chartText(text string, size float32, colorName fyne.ThemeColorName, bold bool) *canvas.Text {
	label := canvas.NewText(text, theme.Color(colorName))
	label.TextSize = size
	label.TextStyle = fyne.TextStyle{Bold: bold}
	return label
}

func lifted(c color.Color) color.Color {
	return kit.Blend(c, color.White, 0.25)
}

func chartTooltip(value, detail string, anchor fyne.Position, bounds fyne.Size) []fyne.CanvasObject {
	caption := theme.CaptionTextSize()
	valueText := chartText(value, theme.TextSize(), theme.ColorNameForeground, true)
	detailText := chartText(detail, caption, theme.ColorNamePlaceHolder, false)

	valueSize := valueText.MinSize()
	detailSize := detailText.MinSize()
	size := fyne.NewSize(
		max(valueSize.Width, detailSize.Width)+2*chartTooltipInset,
		valueSize.Height+detailSize.Height+chartTooltipInset,
	)
	position := fyne.NewPos(
		min(max(anchor.X-size.Width/2, 0), bounds.Width-size.Width),
		max(anchor.Y-size.Height-chartAxisGap, 0),
	)

	background := canvas.NewRectangle(theme.Color(theme.ColorNameOverlayBackground))
	background.CornerRadius = 8
	background.StrokeColor = theme.Color(theme.ColorNameSeparator)
	background.StrokeWidth = 1
	background.Move(position)
	background.Resize(size)

	valueText.Move(position.AddXY(chartTooltipInset, chartTooltipInset/2))
	detailText.Move(position.AddXY(chartTooltipInset, chartTooltipInset/2+valueSize.Height))

	return []fyne.CanvasObject{background, valueText, detailText}
}

type columnChart struct {
	widget.BaseWidget

	values  []float64
	ticks   []string
	details []string
	hovered int

	plotLeft float32
	slot     float32
}

func newColumnChart() *columnChart {
	chart := &columnChart{hovered: -1}
	chart.ExtendBaseWidget(chart)
	return chart
}

func (c *columnChart) SetData(values []float64, ticks, details []string) {
	c.values, c.ticks, c.details = values, ticks, details
	c.hovered = -1
	c.Refresh()
}

func (c *columnChart) indexAt(x float32) int {
	if c.slot <= 0 || x < c.plotLeft {
		return -1
	}

	index := int((x - c.plotLeft) / c.slot)
	if index >= len(c.values) {
		return -1
	}

	return index
}

func (c *columnChart) setHovered(index int) {
	if index != c.hovered {
		c.hovered = index
		c.Refresh()
	}
}

func (c *columnChart) MouseIn(event *desktop.MouseEvent) {
	c.setHovered(c.indexAt(event.Position.X))
}

func (c *columnChart) MouseMoved(event *desktop.MouseEvent) {
	c.setHovered(c.indexAt(event.Position.X))
}

func (c *columnChart) MouseOut() {
	c.setHovered(-1)
}

func (c *columnChart) CreateRenderer() fyne.WidgetRenderer {
	return &columnChartRenderer{chart: c}
}

type columnChartRenderer struct {
	chart   *columnChart
	objects []fyne.CanvasObject
}

func (r *columnChartRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, chartHeadroom+chartPlotHeight+chartAxisGap+theme.CaptionTextSize()*1.5)
}

func (r *columnChartRenderer) Layout(size fyne.Size) {
	c := r.chart
	caption := theme.CaptionTextSize()
	r.objects = nil

	maximum := 0.0
	for _, value := range c.values {
		maximum = max(maximum, value)
	}
	ticks := chartTicks(maximum)
	top := ticks[len(ticks)-1]

	var labelWidth float32
	var tickLabels []*canvas.Text
	for _, tick := range ticks {
		label := chartText(chartTickLabel(tick), caption, theme.ColorNamePlaceHolder, false)
		labelWidth = max(labelWidth, label.MinSize().Width)
		tickLabels = append(tickLabels, label)
	}

	plotLeft := labelWidth + chartLabelGap
	plotTop := float32(chartHeadroom)
	plotBottom := size.Height - caption*1.5 - chartAxisGap
	plotHeight := plotBottom - plotTop
	plotWidth := size.Width - plotLeft
	if plotHeight <= 0 || plotWidth <= 0 || len(c.values) == 0 {
		return
	}

	for i, tick := range ticks {
		y := plotBottom - plotHeight*float32(tick/top)
		line := canvas.NewLine(theme.Color(theme.ColorNameSeparator))
		line.StrokeWidth = 1
		line.Position1 = fyne.NewPos(plotLeft, y)
		line.Position2 = fyne.NewPos(size.Width, y)

		label := tickLabels[i]
		labelSize := label.MinSize()
		label.Move(fyne.NewPos(labelWidth-labelSize.Width, y-labelSize.Height/2))
		r.objects = append(r.objects, line, label)
	}

	slot := plotWidth / float32(len(c.values))
	barWidth := max(min(chartMaxBar, slot-chartBarGap), 1)
	c.plotLeft, c.slot = plotLeft, slot

	if c.hovered >= 0 && c.hovered < len(c.values) {
		wash := canvas.NewRectangle(theme.Color(theme.ColorNameHover))
		wash.CornerRadius = chartBarRadius
		wash.Move(fyne.NewPos(plotLeft+slot*float32(c.hovered), plotTop))
		wash.Resize(fyne.NewSize(slot, plotHeight))
		r.objects = append(r.objects, wash)
	}

	mark := theme.Color(kit.ColorNameChartMark)
	barTops := make([]float32, len(c.values))
	for i, value := range c.values {
		barTops[i] = plotBottom
		if value <= 0 {
			continue
		}

		height := max(plotHeight*float32(value/top), 1)
		fill := mark
		if i == c.hovered {
			fill = lifted(mark)
		}

		bar := canvas.NewRectangle(fill)
		radius := min(chartBarRadius, barWidth/2, height)
		bar.TopLeftCornerRadius = radius
		bar.TopRightCornerRadius = radius
		bar.Move(fyne.NewPos(plotLeft+slot*float32(i)+(slot-barWidth)/2, plotBottom-height))
		bar.Resize(fyne.NewSize(barWidth, height))
		barTops[i] = plotBottom - height
		r.objects = append(r.objects, bar)
	}

	for i, tick := range c.ticks {
		if tick == "" || i >= len(c.values) {
			continue
		}

		label := chartText(tick, caption, theme.ColorNamePlaceHolder, false)
		labelSize := label.MinSize()
		center := plotLeft + slot*(float32(i)+0.5)
		label.Move(fyne.NewPos(min(max(center-labelSize.Width/2, plotLeft), size.Width-labelSize.Width), plotBottom+chartAxisGap))
		r.objects = append(r.objects, label)
	}

	if c.hovered >= 0 && c.hovered < len(c.values) && c.hovered < len(c.details) {
		anchor := fyne.NewPos(plotLeft+slot*(float32(c.hovered)+0.5), barTops[c.hovered])
		r.objects = append(r.objects, chartTooltip(kit.FormatListening(c.values[c.hovered]), c.details[c.hovered], anchor, size)...)
	}
}

func (r *columnChartRenderer) Refresh() {
	r.Layout(r.chart.Size())
	canvas.Refresh(r.chart)
}

func (r *columnChartRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *columnChartRenderer) Destroy() {}

type heatmap struct {
	widget.BaseWidget

	days    []stats.Period
	hovered int

	left   float32
	top    float32
	cell   float32
	stride float32
}

func newHeatmap() *heatmap {
	chart := &heatmap{hovered: -1}
	chart.ExtendBaseWidget(chart)
	return chart
}

func (h *heatmap) SetData(days []stats.Period) {
	h.days = days
	h.hovered = -1
	h.Refresh()
}

func (h *heatmap) columns() int {
	return (len(h.days) + 6) / 7
}

func (h *heatmap) indexAt(position fyne.Position) int {
	if h.stride <= 0 || position.X < h.left || position.Y < h.top {
		return -1
	}

	column := int((position.X - h.left) / h.stride)
	row := int((position.Y - h.top) / h.stride)
	index := column*7 + row
	if row >= 7 || column >= h.columns() || index >= len(h.days) {
		return -1
	}

	return index
}

func (h *heatmap) setHovered(index int) {
	if index != h.hovered {
		h.hovered = index
		h.Refresh()
	}
}

func (h *heatmap) MouseIn(event *desktop.MouseEvent) {
	h.setHovered(h.indexAt(event.Position))
}

func (h *heatmap) MouseMoved(event *desktop.MouseEvent) {
	h.setHovered(h.indexAt(event.Position))
}

func (h *heatmap) MouseOut() {
	h.setHovered(-1)
}

func (h *heatmap) CreateRenderer() fyne.WidgetRenderer {
	return &heatmapRenderer{chart: h}
}

type heatmapRenderer struct {
	chart   *heatmap
	objects []fyne.CanvasObject
}

func (r *heatmapRenderer) labelWidth() float32 {
	return chartText("Mon", theme.CaptionTextSize(), theme.ColorNamePlaceHolder, false).MinSize().Width + chartLabelGap
}

func (r *heatmapRenderer) MinSize() fyne.Size {
	columns := float32(max(r.chart.columns(), 1))
	caption := theme.CaptionTextSize()
	return fyne.NewSize(
		r.labelWidth()+columns*(heatmapMinCell+heatmapGap),
		chartHeadroom+caption*1.5+7*(heatmapCell+heatmapGap),
	)
}

func (r *heatmapRenderer) Layout(size fyne.Size) {
	h := r.chart
	caption := theme.CaptionTextSize()
	r.objects = nil

	columns := h.columns()
	if columns == 0 {
		return
	}

	left := r.labelWidth()
	stride := min((size.Width-left)/float32(columns), heatmapCell+heatmapGap)
	cell := max(stride-heatmapGap, 1)
	top := float32(chartHeadroom) + caption*1.5
	h.left, h.top, h.cell, h.stride = left, top, cell, stride

	for row, name := range []string{"Mon", "", "Wed", "", "Fri", "", ""} {
		if name == "" {
			continue
		}

		label := chartText(name, caption, theme.ColorNamePlaceHolder, false)
		label.Move(fyne.NewPos(0, top+float32(row)*stride+(cell-label.MinSize().Height)/2))
		r.objects = append(r.objects, label)
	}

	lastLabelEnd := float32(-1)
	for column := 0; column < columns; column++ {
		start := h.days[column*7].Start
		if column > 0 && start.Month() == h.days[(column-1)*7].Start.Month() {
			continue
		}

		label := chartText(start.Format("Jan"), caption, theme.ColorNamePlaceHolder, false)
		x := left + float32(column)*stride
		if x < lastLabelEnd+chartLabelGap {
			continue
		}

		label.Move(fyne.NewPos(x, top-caption*1.5))
		lastLabelEnd = x + label.MinSize().Width
		r.objects = append(r.objects, label)
	}

	for i, day := range h.days {
		name := kit.ColorNameChartEmpty
		if level := heatmapLevel(day.Seconds); level > 0 {
			name = kit.ColorNameChartHeat[level-1]
		}

		fill := theme.Color(name)
		if i == h.hovered {
			fill = lifted(fill)
		}

		square := canvas.NewRectangle(fill)
		square.CornerRadius = 3
		square.Move(fyne.NewPos(left+float32(i/7)*stride, top+float32(i%7)*stride))
		square.Resize(fyne.NewSquareSize(cell))
		r.objects = append(r.objects, square)
	}

	if h.hovered >= 0 && h.hovered < len(h.days) {
		day := h.days[h.hovered]
		value := "No listening"
		if day.Seconds > 0 {
			value = kit.FormatListening(day.Seconds)
		}

		anchor := fyne.NewPos(left+float32(h.hovered/7)*stride+cell/2, top+float32(h.hovered%7)*stride)
		r.objects = append(r.objects, chartTooltip(value, day.Start.Format("Mon 2 Jan 2006"), anchor, size)...)
	}
}

func (r *heatmapRenderer) Refresh() {
	r.Layout(r.chart.Size())
	canvas.Refresh(r.chart)
}

func (r *heatmapRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *heatmapRenderer) Destroy() {}

type heatmapLegend struct {
	widget.BaseWidget
}

func newHeatmapLegend() *heatmapLegend {
	legend := &heatmapLegend{}
	legend.ExtendBaseWidget(legend)
	return legend
}

func (l *heatmapLegend) CreateRenderer() fyne.WidgetRenderer {
	return &heatmapLegendRenderer{legend: l}
}

type heatmapLegendRenderer struct {
	legend  *heatmapLegend
	objects []fyne.CanvasObject
	width   float32
	height  float32
}

func (r *heatmapLegendRenderer) build() {
	caption := theme.CaptionTextSize()
	names := append([]fyne.ThemeColorName{kit.ColorNameChartEmpty}, kit.ColorNameChartHeat...)
	labels := []string{"None", "Under 15m", "15–30m", "30–60m", "1h or more"}

	r.objects = nil
	var x float32
	for i, name := range names {
		swatch := canvas.NewRectangle(theme.Color(name))
		swatch.CornerRadius = 3
		swatch.Move(fyne.NewPos(x, (caption*1.4-heatmapMinCell*1.5)/2))
		swatch.Resize(fyne.NewSquareSize(heatmapMinCell * 1.5))
		x += heatmapMinCell*1.5 + 4

		label := chartText(labels[i], caption, theme.ColorNamePlaceHolder, false)
		label.Move(fyne.NewPos(x, 0))
		x += label.MinSize().Width + 12

		r.objects = append(r.objects, swatch, label)
	}

	r.width, r.height = x, caption*1.4
}

func (r *heatmapLegendRenderer) MinSize() fyne.Size {
	r.build()
	return fyne.NewSize(r.width, r.height)
}

func (r *heatmapLegendRenderer) Layout(fyne.Size) {
	r.build()
}

func (r *heatmapLegendRenderer) Refresh() {
	r.build()
	canvas.Refresh(r.legend)
}

func (r *heatmapLegendRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		r.build()
	}
	return r.objects
}

func (r *heatmapLegendRenderer) Destroy() {}

type valueBar struct {
	widget.BaseWidget

	fraction float64
	value    string
}

func newValueBar(fraction float64, value string) *valueBar {
	bar := &valueBar{fraction: fraction, value: value}
	bar.ExtendBaseWidget(bar)
	return bar
}

func (b *valueBar) CreateRenderer() fyne.WidgetRenderer {
	return &valueBarRenderer{bar: b}
}

type valueBarRenderer struct {
	bar     *valueBar
	objects []fyne.CanvasObject
}

func (r *valueBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(100, theme.CaptionTextSize()*1.6)
}

func (r *valueBarRenderer) Layout(size fyne.Size) {
	label := chartText(r.bar.value, theme.CaptionTextSize(), theme.ColorNameForeground, true)
	labelSize := label.MinSize()

	thickness := float32(10)
	length := max((size.Width-labelSize.Width-chartLabelGap)*float32(min(max(r.bar.fraction, 0), 1)), 2)

	bar := canvas.NewRectangle(theme.Color(kit.ColorNameChartMark))
	radius := min(chartBarRadius, length/2)
	bar.TopRightCornerRadius = radius
	bar.BottomRightCornerRadius = radius
	bar.Move(fyne.NewPos(0, (size.Height-thickness)/2))
	bar.Resize(fyne.NewSize(length, thickness))

	label.Move(fyne.NewPos(length+chartLabelGap, (size.Height-labelSize.Height)/2))
	r.objects = []fyne.CanvasObject{bar, label}
}

func (r *valueBarRenderer) Refresh() {
	r.Layout(r.bar.Size())
	canvas.Refresh(r.bar)
}

func (r *valueBarRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *valueBarRenderer) Destroy() {}
