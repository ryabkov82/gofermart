package accrual

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ryabkov82/gofermart/internal/app/config"
	"github.com/ryabkov82/gofermart/internal/app/models"
	"go.uber.org/zap"
)

type taskTracker struct {
	sync.RWMutex
	active map[string]time.Time // orderNumber -> startTime
}

type AccrualWorker struct {
	service        *AccrualService
	logger         *zap.Logger
	workerCount    int
	pollInterval   time.Duration
	batchSize      int
	batchTimeout   time.Duration
	taskQueue      chan OrderTask
	resultQueue    chan models.OrderAccrual
	shutdownChan   chan struct{}
	wg             sync.WaitGroup
	collectedMutex sync.Mutex
	collected      []models.OrderAccrual
	taskTracker    taskTracker
}

type OrderTask struct {
	OrderNumber string
	Status      models.OrderStatus
	UserID      int
}

func NewWorker(service *AccrualService, logger *zap.Logger, cfg config.AccrualConfig) *AccrualWorker {
	worker := &AccrualWorker{
		service:      service,
		logger:       logger,
		workerCount:  cfg.WorkerCount,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		batchTimeout: cfg.BatchTimeout,
		taskQueue:    make(chan OrderTask, cfg.QueueCapacity),
		resultQueue:  make(chan models.OrderAccrual, cfg.QueueCapacity),
		shutdownChan: make(chan struct{}),
		collected:    make([]models.OrderAccrual, 0, cfg.BatchSize),
	}
	// Инициализируем taskTracker
	worker.taskTracker.active = make(map[string]time.Time)

	return worker
}

func (w *AccrualWorker) Run(ctx context.Context) {
	// Запуск воркеров-обработчиков
	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.processWorker(ctx, i)
	}

	// Запуск батчевого агрегатора
	w.wg.Add(1)
	go w.batchAggregator()

	// Главный цикл загрузки задач
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.Shutdown(5 * time.Second)
			return
		case <-ticker.C:
			w.loadTasks(ctx)
		}
	}
}

func (w *AccrualWorker) loadTasks(ctx context.Context) {
	// 1. Получаем текущий набор заказов в обработке
	activeTasks := w.getActiveTasks() // map[string]struct{}

	// 2. Загружаем новые заказы с исключением уже обрабатываемых
	orders, err := w.service.GetOrdersForProcessing(ctx, w.batchSize*2, activeTasks)

	if err != nil {
		w.logger.Error("Error loading pending orders", zap.Error(err))
		return
	}

	// 3. Добавляем в очередь и в активные задачи
	for _, order := range orders {
		select {
		case w.taskQueue <- OrderTask{OrderNumber: order.Number, Status: order.Status, UserID: order.UserID}:
			w.trackActiveTask(order.Number) // Помечаем как "в обработке"
		case <-w.shutdownChan:
			return
			/* ждем пока не освободится место в канале
			default:
				w.logger.Info("Task queue full, skipping")
				return
			*/
		}

	}
}

func (w *AccrualWorker) processWorker(ctx context.Context, workerID int) {
	defer w.wg.Done()

	for {
		select {
		case <-w.shutdownChan:
			return
		case task := <-w.taskQueue:
			// Обработка задачи
			result, err := w.processSingleOrder(ctx, task, workerID)

			if err != nil {
				w.untrackTask(task.OrderNumber)
				continue
			}

			select {
			case w.resultQueue <- result:
			case <-w.shutdownChan:
				return
			}
		}
	}
}

func (w *AccrualWorker) processSingleOrder(ctx context.Context, task OrderTask, workerID int) (models.OrderAccrual, error) {

	accrualInfo, err := w.service.GetOrderInfo(ctx, task.OrderNumber)
	if err != nil {
		if rateLimitErr, ok := err.(*RateLimitError); ok {
			w.logger.Info(fmt.Sprintf("[Worker %d] Rate limit, sleeping for %v", workerID, rateLimitErr.RetryAfter))
			time.Sleep(rateLimitErr.RetryAfter)
		}
		w.logger.Error("failed to get order info", zap.Error(err))
		return models.OrderAccrual{}, err
	}
	if accrualInfo != nil {
		accrualInfo.UserID = task.UserID
		accrualInfo.StatusOld = task.Status
		return *accrualInfo, nil
	} else {
		// нет информации по заказу
		return models.OrderAccrual{}, errors.New("no content")
	}

}

func (w *AccrualWorker) batchAggregator() {
	defer w.wg.Done()

	batchTimer := time.NewTimer(w.batchTimeout)
	defer batchTimer.Stop()

	for {
		select {
		case <-w.shutdownChan:
			w.flushBatch() // Финальный flush при shutdown
			return

		case result := <-w.resultQueue:
			w.collectedMutex.Lock()
			w.collected = append(w.collected, result)

			// Проверяем достижение размера батча
			if len(w.collected) >= w.batchSize {
				w.flushBatch()
				batchTimer.Reset(w.batchTimeout)
			}
			w.collectedMutex.Unlock()

		case <-batchTimer.C:
			w.collectedMutex.Lock()
			if len(w.collected) > 0 {
				w.flushBatch()
			}
			batchTimer.Reset(w.batchTimeout)
			w.collectedMutex.Unlock()
		}
	}
}

func (w *AccrualWorker) flushBatch() {
	if len(w.collected) == 0 {
		return
	}

	// Группируем по пользователям
	userAccruals := make(map[int]float64)
	var orderUpdates []models.Order

	for _, order := range w.collected {
		if order.Accrual > 0 {
			userAccruals[order.UserID] += order.Accrual
		}
		// Обновляем только если статус изменился
		if order.Status != order.StatusOld {
			orderUpdates = append(orderUpdates, models.Order{
				Number:  order.OrderNumber,
				Status:  order.Status,
				Accrual: order.Accrual,
			})
		}
	}

	err := w.service.ProcessBatchAccruals(userAccruals, orderUpdates)
	if err != nil {
		w.logger.Error("Batch processing failed", zap.Error(err))
	} else {
		w.logger.Info("Successfully processed batch", zap.Int("orders", len(w.collected)))
	}

	// Независимо от результата снимаем отметку
	for _, order := range w.collected {
		w.untrackTask(order.OrderNumber)
	}

	// Очищаем собранные данные
	w.collected = w.collected[:0]

}

func (w *AccrualWorker) trackActiveTask(orderNumber string) {
	w.taskTracker.Lock()
	defer w.taskTracker.Unlock()
	w.taskTracker.active[orderNumber] = time.Now()
}

func (w *AccrualWorker) untrackTask(orderNumber string) {
	w.taskTracker.Lock()
	defer w.taskTracker.Unlock()
	delete(w.taskTracker.active, orderNumber)
}

func (w *AccrualWorker) getActiveTasks() []string {
	w.taskTracker.RLock()
	defer w.taskTracker.RUnlock()

	tasks := make([]string, 0, len(w.taskTracker.active))
	for orderNumber := range w.taskTracker.active {
		tasks = append(tasks, orderNumber)
	}
	return tasks
}

// Graceful shutdown:
func (w *AccrualWorker) Shutdown(timeout time.Duration) {

	close(w.shutdownChan)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("Все воркеры завершили работу")
	case <-time.After(timeout):
		w.logger.Info("Таймаут ожидания завершения воркеров")
	}

	// Дополнительно закрываем каналы
	close(w.taskQueue)
	close(w.resultQueue)

	w.logger.Info("Worker shutdown completed")
}
