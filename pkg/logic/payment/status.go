package payment

type Status string

type StatusTransitionPermission = map[Status][]Status

const (
	StatusPending  Status = "pending"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
	StatusRefunded Status = "refunded"
)

var (
	DefaultStatusTransitionPermission = StatusTransitionPermission{
		StatusPending:  {StatusSuccess, StatusFailed, StatusCanceled},
		StatusSuccess:  {StatusRefunded},
		StatusFailed:   {},
		StatusCanceled: {},
		StatusRefunded: {},
	}
	RecoveryStatusTransitionPermission = StatusTransitionPermission{
		StatusPending:  {StatusSuccess, StatusFailed, StatusCanceled},
		StatusSuccess:  {StatusRefunded},
		StatusFailed:   {StatusPending},
		StatusCanceled: {StatusPending},
		StatusRefunded: {StatusSuccess},
	}
)

func (s Status) CanTransitionTo(permission StatusTransitionPermission, target Status) bool {
	permissions, ok := permission[s]
	if !ok {
		return false
	}
	for _, status := range permissions {
		if status == target {
			return true
		}
	}
	return false
}
