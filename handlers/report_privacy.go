package handlers

import (
	"time"

	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
)

// CommunityReportResponse is the limited view of a report that other citizens
// may see. Reporters always receive the full models.Report payload.
type CommunityReportResponse struct {
	ID                 uuid.UUID              `json:"id"`
	Title              string                 `json:"title,omitempty"`
	Latitude           float64                `json:"latitude"`
	Longitude          float64                `json:"longitude"`
	Address            string                 `json:"address,omitempty"`
	Status             models.ReportStatus    `json:"status"`
	Category           *models.IssueCategory  `json:"category,omitempty"`
	Department         *models.Department     `json:"department,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	IsCommunityPreview bool                   `json:"is_community_preview"`
}

func toCommunityReportView(report models.Report) CommunityReportResponse {
	return CommunityReportResponse{
		ID:                 report.ID,
		Title:              report.Title,
		Latitude:           report.Latitude,
		Longitude:          report.Longitude,
		Address:            report.Address,
		Status:             report.Status,
		Category:           report.Category,
		Department:         report.Department,
		CreatedAt:          report.CreatedAt,
		IsCommunityPreview: true,
	}
}

func citizenOwnsReport(userID uuid.UUID, report models.Report) bool {
	return report.UserID == userID
}

func redactCommunityReportsForCitizen(userID uuid.UUID, reports []models.Report) []interface{} {
	out := make([]interface{}, 0, len(reports))
	for _, report := range reports {
		if citizenOwnsReport(userID, report) {
			out = append(out, report)
			continue
		}
		out = append(out, toCommunityReportView(report))
	}
	return out
}
