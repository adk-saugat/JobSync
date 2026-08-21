package sheets

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/option"
	gapi "google.golang.org/api/sheets/v4"

	"github.com/saugatadhikari/jobSync/internal/models"
)

// VisibleHeaders are the columns users see (B–H). Column A is Row ID (hidden).
var VisibleHeaders = []string{
	"Company",
	"Position",
	"Status",
	"Applied At",
	"Interview At",
	"OA At",
	"Notes",
}

// Headers is the full header row written to the sheet (includes hidden Row ID).
var Headers = append([]string{"Row ID"}, VisibleHeaders...)

var statusChoices = []string{
	models.StatusApplied,
	models.StatusOA,
	models.StatusInterview,
	models.StatusRejected,
	models.StatusOffer,
	models.StatusOther,
}

// Row is one tracker row written to Google Sheets.
type Row struct {
	RowID       string
	Company     string
	Position    string
	Status      string
	AppliedAt   string
	InterviewAt string
	OAAt        string
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

// ApplyFormatting styles the tracker and hides the Row ID column.
func (c *Client) ApplyFormatting(ctx context.Context) error {
	sheetID, err := c.sheetID(ctx)
	if err != nil {
		return err
	}

	requests := []*gapi.Request{
		// Freeze header row.
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
		// Hide Row ID column (A) — still used by the app for updates.
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
		// Taller data rows so wrapped text is readable.
		{
			UpdateDimensionProperties: &gapi.UpdateDimensionPropertiesRequest{
				Range: &gapi.DimensionRange{
					SheetId:    sheetID,
					Dimension:  "ROWS",
					StartIndex: 1,
					EndIndex:   2000,
				},
				Properties: &gapi.DimensionProperties{PixelSize: 48},
				Fields:     "pixelSize",
			},
		},
		// Header row height.
		{
			UpdateDimensionProperties: &gapi.UpdateDimensionPropertiesRequest{
				Range: &gapi.DimensionRange{
					SheetId:    sheetID,
					Dimension:  "ROWS",
					StartIndex: 0,
					EndIndex:   1,
				},
				Properties: &gapi.DimensionProperties{PixelSize: 36},
				Fields:     "pixelSize",
			},
		},
		// Header style (full A–H, including hidden Row ID).
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
						BackgroundColor: &gapi.Color{Red: 0.12, Green: 0.16, Blue: 0.23},
						TextFormat: &gapi.TextFormat{
							Bold:            true,
							ForegroundColor: &gapi.Color{Red: 1, Green: 1, Blue: 1},
						},
						HorizontalAlignment: "CENTER",
						VerticalAlignment:   "MIDDLE",
						WrapStrategy:        "WRAP",
					},
				},
				Fields: "userEnteredFormat(backgroundColor,textFormat,horizontalAlignment,verticalAlignment,wrapStrategy)",
			},
		},
		// Data cells: wrap + middle align so content stays visible.
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
						VerticalAlignment:   "MIDDLE",
						HorizontalAlignment: "LEFT",
						WrapStrategy:        "WRAP",
						Padding: &gapi.Padding{
							Top:    4,
							Bottom: 4,
							Left:   6,
							Right:  6,
						},
					},
				},
				Fields: "userEnteredFormat(verticalAlignment,horizontalAlignment,wrapStrategy,padding)",
			},
		},
		// Status column centered.
		{
			RepeatCell: &gapi.RepeatCellRequest{
				Range: &gapi.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      2000,
					StartColumnIndex: 3,
					EndColumnIndex:   4,
				},
				Cell: &gapi.CellData{
					UserEnteredFormat: &gapi.CellFormat{
						HorizontalAlignment: "CENTER",
						VerticalAlignment:   "MIDDLE",
					},
				},
				Fields: "userEnteredFormat(horizontalAlignment,verticalAlignment)",
			},
		},
		// Status dropdown (column D = index 3).
		{
			SetDataValidation: &gapi.SetDataValidationRequest{
				Range: &gapi.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      2000,
					StartColumnIndex: 3,
					EndColumnIndex:   4,
				},
				Rule: &gapi.DataValidationRule{
					Condition: &gapi.BooleanCondition{
						Type:   "ONE_OF_LIST",
						Values: conditionValues(statusChoices...),
					},
					ShowCustomUi: true,
					Strict:       true,
				},
			},
		},
	}

	// Visible column widths: Company, Position, Status, Applied, Interview, OA, Notes.
	// (Index 0 is Row ID — hidden; widths start at column B = index 1.)
	widths := map[int64]int64{
		1: 170, // Company
		2: 240, // Position
		3: 120, // Status
		4: 130, // Applied At
		5: 130, // Interview At
		6: 120, // OA At
		7: 360, // Notes
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

	statusColors := []struct {
		value string
		color *gapi.Color
	}{
		{models.StatusApplied, &gapi.Color{Red: 0.81, Green: 0.89, Blue: 0.98}},
		{models.StatusOA, &gapi.Color{Red: 1.00, Green: 0.90, Blue: 0.75}},
		{models.StatusInterview, &gapi.Color{Red: 0.78, Green: 0.94, Blue: 0.81}},
		{models.StatusRejected, &gapi.Color{Red: 0.98, Green: 0.80, Blue: 0.80}},
		{models.StatusOffer, &gapi.Color{Red: 0.85, Green: 0.82, Blue: 0.95}},
		{models.StatusOther, &gapi.Color{Red: 0.90, Green: 0.90, Blue: 0.90}},
	}
	for i, sc := range statusColors {
		ruleIndex := int64(i)
		requests = append(requests, &gapi.Request{
			AddConditionalFormatRule: &gapi.AddConditionalFormatRuleRequest{
				Index: ruleIndex,
				Rule: &gapi.ConditionalFormatRule{
					Ranges: []*gapi.GridRange{{
						SheetId:          sheetID,
						StartRowIndex:    1,
						EndRowIndex:      2000,
						StartColumnIndex: 3, // Status
						EndColumnIndex:   4,
					}},
					BooleanRule: &gapi.BooleanRule{
						Condition: &gapi.BooleanCondition{
							Type: "TEXT_EQ",
							Values: []*gapi.ConditionValue{
								{UserEnteredValue: sc.value},
							},
						},
						Format: &gapi.CellFormat{
							BackgroundColor: sc.color,
							TextFormat:      &gapi.TextFormat{Bold: true},
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

	a1 := fmt.Sprintf("%s!A%d:H%d", c.sheetName, next, next)
	_, err = c.svc.Spreadsheets.Values.Update(c.spreadsheetID, a1, &gapi.ValueRange{
		Values: [][]any{rowValues(row)},
	}).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("append row: %w", err)
	}
	return nil
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

	a1 := fmt.Sprintf("%s!A%d:H%d", c.sheetName, rowIndex+1, rowIndex+1)
	_, err = c.svc.Spreadsheets.Values.Update(c.spreadsheetID, a1, &gapi.ValueRange{
		Values: [][]any{rowValues(row)},
	}).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("update row: %w", err)
	}
	return nil
}

// FindRowID reports whether a Row ID exists in the hidden column.
func (c *Client) FindRowID(ctx context.Context, rowID string) (bool, error) {
	idx, err := c.findRowIndexByID(ctx, rowID)
	if err != nil {
		return false, err
	}
	return idx >= 0, nil
}

// nextDataRow returns the 1-based sheet row number for the next empty data row.
func (c *Client) nextDataRow(ctx context.Context) (int, error) {
	resp, err := c.svc.Spreadsheets.Values.Get(
		c.spreadsheetID,
		fmt.Sprintf("%s!A:H", c.sheetName),
	).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("list rows: %w", err)
	}
	// Row 1 is headers; first empty row after last non-empty wins.
	last := 1
	for i, row := range resp.Values {
		if rowHasContent(row) {
			last = i + 1 // convert 0-based index to 1-based sheet row
		}
	}
	return last + 1, nil
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
	resp, err := c.svc.Spreadsheets.Values.Get(
		c.spreadsheetID,
		fmt.Sprintf("%s!A:A", c.sheetName),
	).Context(ctx).Do()
	if err != nil {
		return -1, fmt.Errorf("list row ids: %w", err)
	}
	want := strings.TrimSpace(rowID)
	for i, row := range resp.Values {
		if i == 0 {
			continue
		}
		if len(row) > 0 && strings.TrimSpace(fmt.Sprint(row[0])) == want {
			return i, nil
		}
	}
	return -1, nil
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

func conditionValues(values ...string) []*gapi.ConditionValue {
	out := make([]*gapi.ConditionValue, len(values))
	for i, v := range values {
		out[i] = &gapi.ConditionValue{UserEnteredValue: v}
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
