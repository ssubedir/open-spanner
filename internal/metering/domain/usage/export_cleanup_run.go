package usage

import (
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type ExportCleanupRun struct {
	id             string
	workspaceID    string
	expiredBefore  time.Time
	filesDeleted   int
	bytesReclaimed int64
	failures       int
	createdAt      time.Time
}

func NewExportCleanupRun(id, workspaceID string, expiredBefore time.Time, filesDeleted int, bytesReclaimed int64, failures int, createdAt time.Time) (ExportCleanupRun, error) {
	id = strings.TrimSpace(id)
	workspaceID = strings.TrimSpace(workspaceID)
	if id == "" || workspaceID == "" || expiredBefore.IsZero() || createdAt.IsZero() {
		return ExportCleanupRun{}, fmt.Errorf("%w: export cleanup run identity and timestamps are required", domain.ErrInvalidInput)
	}
	if filesDeleted < 0 || bytesReclaimed < 0 || failures < 0 {
		return ExportCleanupRun{}, fmt.Errorf("%w: export cleanup metrics cannot be negative", domain.ErrInvalidInput)
	}
	return ExportCleanupRun{id: id, workspaceID: workspaceID, expiredBefore: expiredBefore.UTC(), filesDeleted: filesDeleted, bytesReclaimed: bytesReclaimed, failures: failures, createdAt: createdAt.UTC()}, nil
}

func (r ExportCleanupRun) ID() string               { return r.id }
func (r ExportCleanupRun) WorkspaceID() string      { return r.workspaceID }
func (r ExportCleanupRun) ExpiredBefore() time.Time { return r.expiredBefore }
func (r ExportCleanupRun) FilesDeleted() int        { return r.filesDeleted }
func (r ExportCleanupRun) BytesReclaimed() int64    { return r.bytesReclaimed }
func (r ExportCleanupRun) Failures() int            { return r.failures }
func (r ExportCleanupRun) CreatedAt() time.Time     { return r.createdAt }
