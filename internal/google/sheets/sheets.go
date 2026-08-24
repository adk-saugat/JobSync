package sheets

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/option"
	gapi "google.golang.org/api/sheets/v4"

	"github.com/saugatadhikari/jobSync/internal/domain"
)

// VisibleHeaders are the columns users see (B–H). Column A is Row ID (hidden).
var VisibleHeaders = []string{
	"Company",
	"Position",
	"Status",
	"Applied At",
	"Interview At",
	"Assessment At",
	"Notes",
}

// Headers is the full header row written to the sheet (includes hidden Row ID).
var Headers = append([]string{"Row ID"}, VisibleHeaders...)

// Row is one tracker row written to Google Sheets.
type Row struct {
	RowID       string
	Company     string
	Position    string
	Status      string
	AppliedAt   string
	InterviewAt string
	OAAt        string // Assessment At column (online assessment date)
	Notes       string
}

// Client talks to the Google Sheets API.
type Client struct {
	svc           *gapi.Service
	spreadsheetID string
	sheetName     string
}

// NewClient builds a Sheets client from an authenticated HTTP client.
func NewClient(ctx context.Context, httpClient *http.Client, spreadsheetID, sheetName string) (*Client, error) {
	svc, err := gapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("sheets service: %w", err)
	}
	if sheetName == "" {
		sheetName = "Applications"
	}
	return &Client{
		svc:           svc,
		spreadsheetID: spreadsheetID,
		sheetName:     sheetName,
	}, nil
}

// CreateTrackerSpreadsheet creates a new spreadsheet with styled headers.
func CreateTrackerSpreadsheet(ctx context.Context, httpClient *http.Client, title, sheetName string) (spreadsheetID string, err error) {
	svc, err := gapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return "", err
	}
	if sheetName == "" {
		sheetName = "Applications"
	}
	if title == "" {
		title = "JobSync Tracker"
	}

	ss, err := svc.Spreadsheets.Create(&gapi.Spreadsheet{
		Properties: &gapi.SpreadsheetProperties{Title: title},
		Sheets: []*gapi.Sheet{{
			Properties: &gapi.SheetProperties{Title: sheetName},
		}},
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create spreadsheet: %w", err)
	}

	c := &Client{svc: svc, spreadsheetID: ss.SpreadsheetId, sheetName: sheetName}
	if err := c.SetupSheet(ctx); err != nil {
		return ss.SpreadsheetId, err
	}
	return ss.SpreadsheetId, nil
}

// SpreadsheetURL returns the browser URL for the spreadsheet.
func SpreadsheetURL(id string) string {
	return fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit", id)
}

// SetupSheet writes headers and applies formatting (safe to re-run).
func (c *Client) SetupSheet(ctx context.Context) error {
	if err := c.EnsureHeaders(ctx); err != nil {
		return err
	}
	return c.ApplyFormatting(ctx)
}

// EnsureHeaders writes the header row. If an old layout is detected, clears the tab first.
func (c *Client) EnsureHeaders(ctx context.Context) error {
	rangeA1 := fmt.Sprintf("%s!A1:H1", c.sheetName)
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, rangeA1).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("read headers: %w", err)
	}
	if len(resp.Values) > 0 && headerMatches(resp.Values[0]) {
		return nil
	}

	if len(resp.Values) > 0 && !headerMatches(resp.Values[0]) {
		clearRange := fmt.Sprintf("%s!A1:Z1000", c.sheetName)
		_, err = c.svc.Spreadsheets.Values.Clear(c.spreadsheetID, clearRange, &gapi.ClearValuesRequest{}).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("clear old sheet layout: %w", err)
		}
	}

	_, err = c.svc.Spreadsheets.Values.Update(c.spreadsheetID, rangeA1, &gapi.ValueRange{
		Values: [][]any{toAnySlice(Headers)},
	}).ValueInputOption("RAW").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("write headers: %w", err)
	}
	return nil
}

// ApplyFormatting keeps the sheet minimal: hidden Row ID, plain header, whole-row status colors.
func (c *Client) ApplyFormatting(ctx context.Context) error {
	sheetID, err := c.sheetID(ctx)
	if err != nil {
		return err
	}

	requests := []*gapi.Request{
		{
			UpdateSheetProperties: &gapi.UpdateSheetPropertiesRequest{
				Properties: &gapi.SheetProperties{
					SheetId: sheetID,
					GridProperties: &gapi.GridProperties{
						FrozenRowCount: 1,
					},
				},
				Fields: "gridProperties.frozenRowCount",
			},
		},
		// Hide Row ID column (A).
		{
			UpdateDimensionProperties: &gapi.UpdateDimensionPropertiesRequest{
				Range: &gapi.DimensionRange{
					SheetId:    sheetID,
					Dimension:  "COLUMNS",
					StartIndex: 0,
					EndIndex:   1,
				},
				Properties: &gapi.DimensionProperties{HiddenByUser: true},
				Fields:     "hiddenByUser",
			},
		},
		// Plain header: bold only, no fill color.
		{
			RepeatCell: &gapi.RepeatCellRequest{
				Range: &gapi.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    0,
					EndRowIndex:      1,
					StartColumnIndex: 0,
					EndColumnIndex:   int64(len(Headers)),
				},
				Cell: &gapi.CellData{
					UserEnteredFormat: &gapi.CellFormat{
						BackgroundColor: &gapi.Color{Red: 1, Green: 1, Blue: 1},
						TextFormat: &gapi.TextFormat{
							Bold:            true,
							ForegroundColor: &gapi.Color{Red: 0, Green: 0, Blue: 0},
						},
						VerticalAlignment: "MIDDLE",
					},
				},
				Fields: "userEnteredFormat(backgroundColor,textFormat,verticalAlignment)",
			},
		},
		// Clear any status dropdown from earlier layouts.
		{
			SetDataValidation: &gapi.SetDataValidationRequest{
				Range: &gapi.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      2000,
					StartColumnIndex: 3,
					EndColumnIndex:   4,
				},
			},
		},
		// Readable row height + wrap for notes.
		{
			UpdateDimensionProperties: &gapi.UpdateDimensionPropertiesRequest{
				Range: &gapi.DimensionRange{
					SheetId:    sheetID,
					Dimension:  "ROWS",
					StartIndex: 1,
					EndIndex:   2000,
				},
				Properties: &gapi.DimensionProperties{PixelSize: 36},
				Fields:     "pixelSize",
			},
		},
		{
			RepeatCell: &gapi.RepeatCellRequest{
				Range: &gapi.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      2000,
					StartColumnIndex: 1,
					EndColumnIndex:   int64(len(Headers)),
				},
				Cell: &gapi.CellData{
					UserEnteredFormat: &gapi.CellFormat{
						WrapStrategy:      "WRAP",
						VerticalAlignment: "MIDDLE",
					},
				},
				Fields: "userEnteredFormat(wrapStrategy,verticalAlignment)",
			},
		},
	}

	widths := map[int64]int64{
		1: 160, // Company
		2: 220, // Position
		3: 110, // Status
		4: 110, // Applied At
		5: 110, // Interview At
		6: 120, // Assessment At
		7: 300, // Notes
	}
	for col, w := range widths {
		requests = append(requests, &gapi.Request{
			UpdateDimensionProperties: &gapi.UpdateDimensionPropertiesRequest{
				Range: &gapi.DimensionRange{
					SheetId:    sheetID,
					Dimension:  "COLUMNS",
					StartIndex: col,
					EndIndex:   col + 1,
				},
				Properties: &gapi.DimensionProperties{PixelSize: w},
				Fields:     "pixelSize",
			},
		})
	}

	meta, err := c.svc.Spreadsheets.Get(c.spreadsheetID).
		Fields("sheets(properties(sheetId,title),conditionalFormats)").
		Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("read conditional formats: %w", err)
	}
	for _, sh := range meta.Sheets {
		if sh.Properties == nil || sh.Properties.SheetId != sheetID {
			continue
		}
		for i := len(sh.ConditionalFormats) - 1; i >= 0; i-- {
			idx := int64(i)
			requests = append(requests, &gapi.Request{
				DeleteConditionalFormatRule: &gapi.DeleteConditionalFormatRuleRequest{
					SheetId: sheetID,
					Index:   idx,
				},
			})
		}
	}

	// Whole-row colors from Status (column D). applied/other stay uncolored.
	rowColors := []struct {
		status string
		color  *gapi.Color
	}{
		{domain.StatusRejected, &gapi.Color{Red: 0.96, Green: 0.80, Blue: 0.80}},    // red
		{domain.StatusInterview, &gapi.Color{Red: 0.80, Green: 0.90, Blue: 0.98}},   // blue
		{domain.StatusAssessment, &gapi.Color{Red: 1.00, Green: 0.92, Blue: 0.78}},  // amber
		{domain.StatusAccepted, &gapi.Color{Red: 0.78, Green: 0.93, Blue: 0.82}},    // green
		// Back-compat if old values remain in the sheet.
		{"oa", &gapi.Color{Red: 1.00, Green: 0.92, Blue: 0.78}},
		{"offer", &gapi.Color{Red: 0.78, Green: 0.93, Blue: 0.82}},
	}

	dataRange := &gapi.GridRange{
		SheetId:          sheetID,
		StartRowIndex:    1,
		EndRowIndex:      2000,
		StartColumnIndex: 0,
		EndColumnIndex:   int64(len(Headers)),
	}
	for i, rc := range rowColors {
		formula := fmt.Sprintf(`=$D2="%s"`, rc.status)
		ruleIndex := int64(i)
		requests = append(requests, &gapi.Request{
			AddConditionalFormatRule: &gapi.AddConditionalFormatRuleRequest{
				Index: ruleIndex,
				Rule: &gapi.ConditionalFormatRule{
					Ranges: []*gapi.GridRange{dataRange},
					BooleanRule: &gapi.BooleanRule{
						Condition: &gapi.BooleanCondition{
							Type: "CUSTOM_FORMULA",
							Values: []*gapi.ConditionValue{
								{UserEnteredValue: formula},
							},
						},
						Format: &gapi.CellFormat{
							BackgroundColor: rc.color,
						},
					},
				},
			},
		})
	}

	_, err = c.svc.Spreadsheets.BatchUpdate(c.spreadsheetID, &gapi.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("apply formatting: %w", err)
	}
	return nil
}

// AppendRow adds a new data row at the bottom.
// Uses an explicit Update (not Append) so the hidden Row ID column is always written.
// Google Sheets' Append API detects "tables" from visible columns and can skip hidden col A.
func (c *Client) AppendRow(ctx context.Context, row Row) error {
	if strings.TrimSpace(row.RowID) == "" {
		return fmt.Errorf("append row: RowID is required")
	}

	next, err := c.nextDataRow(ctx)
	if err != nil {
		return err
	}
	return c.writeAtSheetRow(ctx, next, row)
}

// UpdateRowByID finds the row by hidden Row ID and updates it.
func (c *Client) UpdateRowByID(ctx context.Context, row Row) error {
	if strings.TrimSpace(row.RowID) == "" {
		return fmt.Errorf("update row: RowID is required")
	}
	rowIndex, err := c.findRowIndexByID(ctx, row.RowID)
	if err != nil {
		return err
	}
	if rowIndex < 0 {
		return fmt.Errorf("row id %q not found", row.RowID)
	}
	return c.writeAtSheetRow(ctx, rowIndex+1, row)
}

// WriteRow updates an existing row (by Row ID, else by company+position) or appends.
// This prevents duplicate sheet rows when the local/cloud DB is out of sync with the sheet.
func (c *Client) WriteRow(ctx context.Context, row Row) error {
	if strings.TrimSpace(row.RowID) == "" {
		return fmt.Errorf("write row: RowID is required")
	}
	if strings.TrimSpace(row.Company) == "" || strings.TrimSpace(row.Position) == "" {
		return fmt.Errorf("write row: company and position are required")
	}

	values, err := c.readAllValues(ctx)
	if err != nil {
		return err
	}

	if idx := indexByRowID(values, row.RowID); idx >= 0 {
		return c.writeAtSheetRow(ctx, idx+1, row)
	}
	if idx := indexByCompanyPosition(values, row.Company, row.Position); idx >= 0 {
		return c.writeAtSheetRow(ctx, idx+1, row)
	}
	return c.writeAtSheetRow(ctx, nextEmptySheetRow(values), row)
}

// FindByCompanyAndPosition returns the first sheet row matching company+position
// (case-insensitive). When duplicates exist, the earliest row wins.
func (c *Client) FindByCompanyAndPosition(ctx context.Context, company, position string) (*Row, error) {
	values, err := c.readAllValues(ctx)
	if err != nil {
		return nil, err
	}
	idx := indexByCompanyPosition(values, company, position)
	if idx < 0 {
		return nil, nil
	}
	parsed := parseDataRow(values[idx])
	return &parsed, nil
}

// FindRowID reports whether a Row ID exists in the hidden column.
func (c *Client) FindRowID(ctx context.Context, rowID string) (bool, error) {
	idx, err := c.findRowIndexByID(ctx, rowID)
	if err != nil {
		return false, err
	}
	return idx >= 0, nil
}

func (c *Client) writeAtSheetRow(ctx context.Context, sheetRow1Based int, row Row) error {
	a1 := fmt.Sprintf("%s!A%d:H%d", c.sheetName, sheetRow1Based, sheetRow1Based)
	_, err := c.svc.Spreadsheets.Values.Update(c.spreadsheetID, a1, &gapi.ValueRange{
		Values: [][]any{rowValues(row)},
	}).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("write row %d: %w", sheetRow1Based, err)
	}
	return nil
}

func (c *Client) readAllValues(ctx context.Context) ([][]any, error) {
	resp, err := c.svc.Spreadsheets.Values.Get(
		c.spreadsheetID,
		fmt.Sprintf("%s!A:H", c.sheetName),
	).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list rows: %w", err)
	}
	return resp.Values, nil
}

// nextDataRow returns the 1-based sheet row number for the next empty data row.
func (c *Client) nextDataRow(ctx context.Context) (int, error) {
	values, err := c.readAllValues(ctx)
	if err != nil {
		return 0, err
	}
	return nextEmptySheetRow(values), nil
}

func nextEmptySheetRow(values [][]any) int {
	// Row 1 is headers; first empty row after last non-empty wins.
	last := 1
	for i, row := range values {
		if rowHasContent(row) {
			last = i + 1 // convert 0-based index to 1-based sheet row
		}
	}
	return last + 1
}

func indexByRowID(values [][]any, rowID string) int {
	want := strings.TrimSpace(rowID)
	if want == "" {
		return -1
	}
	for i, row := range values {
		if i == 0 {
			continue
		}
		if cellString(row, 0) == want {
			return i
		}
	}
	return -1
}

func indexByCompanyPosition(values [][]any, company, position string) int {
	want := companyPositionKey(company, position)
	if want == "\x00" {
		return -1
	}
	for i, row := range values {
		if i == 0 {
			continue
		}
		if companyPositionKey(cellString(row, 1), cellString(row, 2)) == want {
			return i
		}
	}
	return -1
}

func companyPositionKey(company, position string) string {
	return strings.ToLower(strings.TrimSpace(company)) + "\x00" + strings.ToLower(strings.TrimSpace(position))
}

func cellString(row []any, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(row[i]))
}

func parseDataRow(row []any) Row {
	return Row{
		RowID:       cellString(row, 0),
		Company:     cellString(row, 1),
		Position:    cellString(row, 2),
		Status:      cellString(row, 3),
		AppliedAt:   cellString(row, 4),
		InterviewAt: cellString(row, 5),
		OAAt:        cellString(row, 6),
		Notes:       cellString(row, 7),
	}
}

func rowHasContent(row []any) bool {
	for _, cell := range row {
		if strings.TrimSpace(fmt.Sprint(cell)) != "" {
			return true
		}
	}
	return false
}

func (c *Client) findRowIndexByID(ctx context.Context, rowID string) (int, error) {
	values, err := c.readAllValues(ctx)
	if err != nil {
		return -1, err
	}
	return indexByRowID(values, rowID), nil
}

func (c *Client) sheetID(ctx context.Context) (int64, error) {
	ss, err := c.svc.Spreadsheets.Get(c.spreadsheetID).
		Fields("sheets.properties").
		Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("get spreadsheet: %w", err)
	}
	for _, sh := range ss.Sheets {
		if sh.Properties != nil && sh.Properties.Title == c.sheetName {
			return sh.Properties.SheetId, nil
		}
	}
	return 0, fmt.Errorf("sheet tab %q not found", c.sheetName)
}

func rowValues(row Row) []any {
	return []any{
		row.RowID,
		row.Company,
		row.Position,
		row.Status,
		row.AppliedAt,
		row.InterviewAt,
		row.OAAt,
		row.Notes,
	}
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func headerMatches(row []any) bool {
	if len(row) < len(Headers) {
		return false
	}
	for i, h := range Headers {
		if strings.TrimSpace(fmt.Sprint(row[i])) != h {
			return false
		}
	}
	return true
}
