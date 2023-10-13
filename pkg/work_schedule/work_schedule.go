package work_schedule

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/evgeniums/go-backend-helpers/pkg/app_context"
	"github.com/evgeniums/go-backend-helpers/pkg/background_worker"
	"github.com/evgeniums/go-backend-helpers/pkg/cache"
	"github.com/evgeniums/go-backend-helpers/pkg/common"
	"github.com/evgeniums/go-backend-helpers/pkg/config/object_config"
	"github.com/evgeniums/go-backend-helpers/pkg/crud"
	"github.com/evgeniums/go-backend-helpers/pkg/db"
	"github.com/evgeniums/go-backend-helpers/pkg/multitenancy"
	"github.com/evgeniums/go-backend-helpers/pkg/multitenancy/app_with_multitenancy"
	"github.com/evgeniums/go-backend-helpers/pkg/op_context"
	"github.com/evgeniums/go-backend-helpers/pkg/op_context/default_op_context"
	"github.com/evgeniums/go-backend-helpers/pkg/utils"
)

type PostMode int

const (
	SCHEDULE PostMode = 0
	DIRECT   PostMode = 1
	QUEUED   PostMode = 2
)

type Work interface {
	common.Object

	GetReferenceType() string
	SetReferenceType(string)

	GetReferenceId() string
	SetReferenceId(string)

	GetNextTime() time.Time
	SetNextTime(time.Time)
	ResetNextTime()

	GetDelay() int
	SetDelay(int)

	SetLock(cache.Lock)
	GetLock() cache.Lock

	SetNoDb(enable bool)
	IsNoDb() bool
}

type WorkBuilder[T Work] func() T

type WorkInvoker[T Work] func(ctx op_context.Context, work T, postMode PostMode, tenancy ...multitenancy.Tenancy) error

type WorkScheduler[T Work] interface {
	NewWork(referenceId string, referenceType string) T
	AcquireWork(ctx op_context.Context, work T) error
	ReleaseWork(ctx op_context.Context, work T) error
	PostWork(ctx op_context.Context, work T, postMode PostMode, tenancy ...multitenancy.Tenancy) error
	RemoveWork(ctx op_context.Context, referenceId string) error
}

type WorkSchedulerBase[T Work] struct {
	workBuilder WorkBuilder[T]
}

func (s *WorkSchedulerBase[T]) Construct(workBuilder WorkBuilder[T]) {
	s.workBuilder = workBuilder
}

func (s *WorkSchedulerBase[T]) NewWork(referenceId string, referenceType string) T {
	w := s.workBuilder()
	w.InitObject()
	w.SetReferenceId(referenceId)
	w.SetReferenceType(referenceType)
	return w
}

func (s *WorkSchedulerBase[T]) AcquireWork(ctx op_context.Context, work T) error {
	return nil
}

func (s *WorkSchedulerBase[T]) ReleaseWork(ctx op_context.Context, work T) error {
	return nil
}

func (s *WorkSchedulerBase[T]) PostWork(ctx op_context.Context, work T, postMode PostMode, tenancy ...multitenancy.Tenancy) error {
	return nil
}

func (s *WorkSchedulerBase[T]) RemoveWork(ctx op_context.Context, referenceId string) error {
	return nil
}

type WorkRunner[T Work] interface {
	Run(ctx op_context.Context, work T) (bool, error)
}

type WorkBase struct {
	common.ObjectBase
	ReferenceId   string    `json:"reference_id" gorm:"index;index:,unique,composite:ref"`
	ReferenceType string    `json:"reference_type" gorm:"index;index:,unique,composite:ref"`
	NextTime      time.Time `json:"next_time" gorm:"index"`

	lock  cache.Lock `json:"-" gorm:"-:all"`
	delay int        `json:"-" gorm:"-:all"`
	noDb  bool       `json:"-" gorm:"-:all"`
}

func (w *WorkBase) GetReferenceType() string {
	return w.ReferenceType
}

func (w *WorkBase) SetReferenceType(referenceType string) {
	w.ReferenceType = referenceType
}

func (w *WorkBase) GetReferenceId() string {
	return w.ReferenceId
}

func (w *WorkBase) SetReferenceId(referenceId string) {
	w.ReferenceId = referenceId
}

func (w *WorkBase) GetNextTime() time.Time {
	return w.NextTime
}

func (w *WorkBase) SetNextTime(nextTime time.Time) {
	w.NextTime = nextTime
}

func (w *WorkBase) ResetNextWorkTime() {
	w.SetNextTime(time.Time{})
	w.SetDelay(0)
}

func (w *WorkBase) GetLock() cache.Lock {
	return w.lock
}

func (w *WorkBase) SetLock(lock cache.Lock) {
	w.lock = lock
}

func (w *WorkBase) GetDelay() int {
	return w.delay
}

func (w *WorkBase) SetDelay(delay int) {
	w.delay = delay
}

func (w *WorkBase) SetNoDb(enable bool) {
	w.noDb = enable
}

func (w *WorkBase) IsNoDb() bool {
	return w.noDb
}

type WorkScheduleConfig struct {
	PARALLEL_JOBS               int `default:"8"`
	BUCKET_SIZE                 int `default:"32"`
	INVOKATION_INTERVAL_SECONDS int `default:"300"`
	HOLD_WORK_SECONDS           int `default:"900"`
	LOCK_TTL_SECONDS            int `default:"300"`
	PERIOD                      int `default:"5"`
}

type workItem[T Work] struct {
	work    T
	tenancy multitenancy.Tenancy
}

type WorkSchedule[T Work] struct {
	WorkScheduleConfig
	app_context.WithAppBase
	crud.WithCRUDBase
	background_worker.JobRunnerBase

	WorkSchedulerBase[T]

	name string

	runningWorkCount atomic.Int32
	workQueueSize    atomic.Int32

	workRunner WorkRunner[T]

	queue chan workItem[T]

	running atomic.Bool
	invoker WorkInvoker[T]

	locker cache.Locker
}

type Config[T Work] struct {
	WorkBuilder WorkBuilder[T]
	WorkRunner  WorkRunner[T]
	WorkInvoker WorkInvoker[T]
}

func NewWorkSchedule[T Work](name string, config Config[T], cruds ...crud.CRUD) *WorkSchedule[T] {
	s := &WorkSchedule[T]{
		name:       name,
		workRunner: config.WorkRunner,
	}
	s.WorkSchedulerBase.Construct(config.WorkBuilder)
	s.WithCRUDBase.Construct(cruds...)
	s.queue = make(chan workItem[T])
	s.invoker = config.WorkInvoker
	if s.invoker == nil {
		s.invoker = s.InvokeWork
	}
	return s
}

func (s *WorkSchedule[T]) Config() interface{} {
	return &s.WorkScheduleConfig
}

func (s *WorkSchedule[T]) Init(app app_context.Context, configPath ...string) error {

	s.WithAppBase.Init(app)

	err := object_config.LoadLogValidateApp(app, s, "work_schedule", configPath...)
	if err != nil {
		return app.Logger().PushFatalStack("failed to load configuration of WorkSchedule", err)
	}

	for i := 0; i < s.PARALLEL_JOBS; i++ {
		go s.worker()
	}

	return nil
}

func (s *WorkSchedule[T]) StopJob() {
	close(s.queue)
}

func (s *WorkSchedule[T]) RunJob() {
	s.ProcessWorks()
}

func (s *WorkSchedule[T]) SetRunner(runner WorkRunner[T]) {
	s.workRunner = runner
}

func (s *WorkSchedule[T]) AcquireWork(ctx op_context.Context, work T) error {

	// setup
	var err error
	c := ctx.TraceInMethod("WorkSchedule.AcquireWork")
	defer ctx.TraceOutMethod()

	key := fmt.Sprintf("work_lock_%s", work.GetReferenceId())

	lock, err := s.locker.Lock(key, time.Duration(s.LOCK_TTL_SECONDS))
	if err != nil {
		c.SetLoggerField("work_reference_id", work.GetReferenceId())
		c.SetMessage("failed to lock work")
		return c.SetError(err)
	}
	s.runningWorkCount.Add(1)
	work.SetLock(lock)

	return nil
}

func (s *WorkSchedule[T]) ReleaseWork(ctx op_context.Context, work T) error {

	var err error
	c := ctx.TraceInMethod("WorkSchedule.ReleaseWork")
	defer ctx.TraceOutMethod()

	lock := work.GetLock()
	if lock != nil {
		s.runningWorkCount.Add(-1)
		err = lock.Release()
		if err != nil {
			c.SetLoggerField("work_reference_id", work.GetReferenceId())
			c.SetMessage("failed to release work")
			return c.SetError(err)
		}
	}

	return nil
}

func (s *WorkSchedule[T]) SetNextWorkTime(work T, reset ...bool) {
	defaultNextTime := utils.OptionalArg(false, reset...)
	if defaultNextTime || work.GetDelay() == 0 && work.GetNextTime() == utils.TimeNil {
		work.SetNextTime(time.Now().Add(time.Second * time.Duration(s.INVOKATION_INTERVAL_SECONDS)))
	} else if work.GetNextTime() == utils.TimeNil && work.GetDelay() != 0 {
		work.SetNextTime(time.Now().Add(time.Second * time.Duration(work.GetDelay())))
	}
}

func (s *WorkSchedule[T]) PostWork(ctx op_context.Context, work T, postMode PostMode, tenancy ...multitenancy.Tenancy) error {

	// setup
	var err error
	c := ctx.TraceInMethod("WorkSchedule.PostWork")
	defer ctx.TraceOutMethod()

	// set next time
	if postMode == SCHEDULE {
		s.SetNextWorkTime(work)
	} else {
		work.SetNextTime(time.Now().Add(time.Second * time.Duration(s.HOLD_WORK_SECONDS)))
	}

	// create work in database
	if !work.IsNoDb() {
		_, err = s.CRUD().CreateDup(ctx, work)
		if err != nil {
			c.SetLoggerField("work_reference_id", work.GetReferenceId())
			c.SetMessage("failed to save work in database")
			return c.SetError(err)
		}
	}

	if postMode != SCHEDULE {

		if ctx.DbTransaction() != nil {
			c.Logger().Error("incompatible mode for calling inside transaction", nil)
			return nil
		}

		// read work from database
		dbWork := s.workBuilder()
		found, err := s.CRUD().Read(ctx, db.Fields{"reference_id": work.GetReferenceId()}, dbWork)
		if err != nil {
			c.SetMessage("failed to read work from database")
			return c.SetError(err)
		}
		if !found {
			// no work in database
			return nil
		}

		// invoke work
		err = s.invoker(ctx, dbWork, postMode, tenancy...)
		if err != nil {
			c.SetMessage("failed to invoke work")
			return nil
		}
	}

	// done
	return nil
}

func (s *WorkSchedule[T]) RemoveWork(ctx op_context.Context, referenceId string) error {

	// setup
	var err error
	c := ctx.TraceInMethod("WorkSchedule.AddWork")
	defer ctx.TraceOutMethod()

	// delete from database
	err = s.CRUD().DeleteByFields(ctx, db.Fields{"reference_id": referenceId}, s.workBuilder())
	if err != nil {
		c.SetLoggerField("work_reference_id", referenceId)
		c.SetMessage("failed to delete work from database")
		return c.SetError(err)
	}

	// done
	return nil
}

func (s *WorkSchedule[T]) ProcessWorks() {

	if !s.running.CompareAndSwap(false, true) {
		return
	}
	defer s.running.Store(false)

	// TODO support multitenancy

	ctx := default_op_context.BackgroundOpContext(s.App(), s.name)
	defer ctx.Close()
	c := ctx.TraceInMethod("WorkSchedule.ProcessWorks")

	filter := db.NewFilter()

	// prepare filter
	filter.AddInterval("next_time", nil, time.Now())
	filter.SetSorting("next_time", db.SORT_ASC)

	// process works
	for {

		// check if runner is stopped
		if s.Stopper().IsStopped() {
			break
		}

		// check number of works currently pending or being processed
		filter.Limit = s.BUCKET_SIZE - int(s.runningWorkCount.Load()) - int(s.workQueueSize.Load())
		if filter.Limit <= 0 {
			c.Logger().Info("all bucket size is used, skipping")
			break
		}

		// read works from database
		var works []T
		handler := func() error {

			var works1 []T
			_, err := s.CRUD().List(ctx, filter, &works1)
			if err != nil {
				c.SetMessage("failed to read works from database 1")
				return err
			}

			// hold works
			nextTime := time.Now().Add(time.Second * time.Duration(s.HOLD_WORK_SECONDS))
			for _, w := range works1 {
				dbWork := s.workBuilder()
				found, err := s.CRUD().ReadForUpdate(ctx, db.Fields{"id": w.GetID()}, dbWork)
				if err != nil {
					c.SetMessage("failed to read work for hold from database")
					return err
				}
				if found {
					err = s.CRUD().Update(ctx, dbWork, db.Fields{"next_time": nextTime})
					if err != nil {
						c.SetLoggerField("work_reference_id", dbWork.GetReferenceId())
						c.SetMessage("failed hold work in database")
						return err
					}
				}
			}

			// read updated works
			_, err = s.CRUD().List(ctx, filter, &works)
			if err != nil {
				c.SetMessage("failed to read works from database 2")
				return err
			}

			// done
			return nil
		}
		err := op_context.ExecDbTransaction(ctx, handler)
		if err != nil {
			c.SetError(err)
			break
		}

		// stop cycle if there are no works
		if len(works) == 0 || s.Stopper().IsStopped() {
			break
		}

		// enqueu works to workers
		for _, work := range works {
			if s.Stopper().IsStopped() {
				break
			}
			// TODO support multitenancy
			s.enqueuWork(work, nil)
		}
	}
}

func (s *WorkSchedule[T]) DoWork(ctx op_context.Context, work T) error {

	// setup
	var err error
	releaseWork := false
	ctx.SetLoggerField("work_reference_id", work.GetReferenceId())
	c := ctx.TraceInMethod("WorkSchedule.DoWork")
	onExit := func() {

		if releaseWork {
			s.ReleaseWork(ctx, work)
		}
		if err != nil {
			c.SetError(err)
		}

		ctx.TraceOutMethod()
	}
	defer onExit()

	// check work runner
	if s.workRunner == nil {
		err = errors.New("invalid work runner")
		return err
	}

	// acquire work
	s.AcquireWork(ctx, work)
	if err != nil {
		return err
	}
	releaseWork = true

	// run work
	work.ResetNextTime()
	done, err := s.workRunner.Run(ctx, work)
	s.SetNextWorkTime(work)
	updateProcessedWork := func() error {

		// read work from database
		dbWork := s.workBuilder()
		found, err := s.CRUD().ReadForUpdate(ctx, db.Fields{"id": work.GetID()}, dbWork)
		if err != nil {
			c.SetMessage("failed to read processed work from database")
			return err
		}
		if !found {
			// no need to update work in database
			return nil
		}

		// if done then just delete work
		if done {
			err = s.CRUD().Delete(ctx, dbWork)
			if err != nil {
				c.SetMessage("failed to delete processed work from database")
				return err
			}
			return nil
		}

		// set next time
		err = s.CRUD().Update(ctx, dbWork, db.Fields{"next_time": work.GetNextTime()})
		if err != nil {
			c.SetMessage("failed to save next work time in database")
			return err
		}

		// done
		return nil
	}
	err1 := op_context.ExecDbTransaction(ctx, updateProcessedWork)
	if err1 != nil {
		if err == nil {
			err = err1
			return err
		}
		c.Logger().Error("failed to update processed work", err1)
		return err
	}

	// done
	return nil
}

func (s *WorkSchedule[T]) InvokeWork(ctx op_context.Context, work T, postMode PostMode, tenancy ...multitenancy.Tenancy) error {

	c := ctx.TraceInMethod("WorkSchedule.InvokeWork")
	defer ctx.TraceOutMethod()

	switch postMode {
	case DIRECT:
		// TODO support multitenancy
		err := s.DoWork(ctx, work)
		if err != nil {
			return c.SetError(err)
		}
	case QUEUED:
		s.enqueuWork(work, tenancy...)
	}
	return nil
}

func (s *WorkSchedule[T]) worker() {
	for work := range s.queue {

		s.workQueueSize.Add(-1)

		if s.Stopper().IsStopped() {
			break
		}

		if work.tenancy != nil {
			ctx := app_with_multitenancy.BackgroundOpContext(s.App(), work.tenancy, s.name)
			s.DoWork(ctx, work.work)
		} else {
			ctx := default_op_context.BackgroundOpContext(s.App(), s.name)
			s.DoWork(ctx, work.work)
		}

		go s.ProcessWorks()
	}
}

func (s *WorkSchedule[T]) enqueuWork(work T, tenancy ...multitenancy.Tenancy) {
	s.workQueueSize.Add(1)
	s.queue <- workItem[T]{work: work, tenancy: utils.OptionalArg(nil, tenancy...)}
}
