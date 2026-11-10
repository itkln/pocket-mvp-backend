> Статус: эксперимент отключён. Публичный каталог и бронирование доступны только для ресторанов. API записи на услуги снят с маршрутизации. Таблицы и ранее созданные данные сохранены для возможной переработки; миграции 19–20 не откатываются с удалением данных.

# Service appointments

Individual appointments are available for yoga/pilates, masterclasses and beauty.

- `bookable_services` stores each venue's service, price, currency and duration.
- `reservation_services` snapshots the selected service when a reservation is confirmed; later edits do not change the booked price or duration.
- Existing venue layout/table records represent interchangeable staff/workstation resources. Each example has one specialist. Group sessions, per-specialist qualifications and recurring class timetables are not implemented.
- The existing availability calculator uses the service duration, a 30-minute start interval, 09:00–22:00 venue-local working hours and a 30-day booking horizon. One appointment reserves one resource for one guest.
- POST validates the current service, price and slot again. Venue-scoped transaction locks serialize resource selection; the existing exclusion constraint prevents overlapping reservations.
- Idempotency includes normalized input and authenticated identity. A retry with the same key returns the confirmed reservation; changed payloads or cancelled bookings conflict.
- Signed-in bookings use the existing customer reservation history/cancellation. Guests receive a confirmation code. Payment is at the venue; no card payment is performed.
- Notes are limited to 500 Unicode code points on the server. The reservation notes include the service name for compatibility with existing history screens.

## API

- `GET /api/v1/public/venues/{venueID}/services`
- `GET /api/v1/public/venues/{venueID}/service-availability?service_id=UUID&date=YYYY-MM-DD`
- `POST /api/v1/public/venues/{venueID}/appointments` with `service_id`, `starts_at`, `guest_name`, `guest_phone`, `notes`, `expected_price_minor`, `idempotency_key`. Identity is obtained only from the authenticated session.

## Demo fixtures

Apply migration 19, then run `scripts/seed-service-examples.sql` against the local demo database. This additive, repeatable script creates six clearly labelled demo venues and 13 services. It requires the existing demo owner and never resets restaurants, bookings or orders. Cover illustrations use the local category assets.

## Checks

`./scripts/test-integration.sh ./...` uses an isolated PostgreSQL database. Appointment integration tests cover repeatable seeding, service duration, stale price rejection, simultaneous requests, idempotent retries, cross-venue isolation and releasing cancelled time slots. Frontend tests exercise service selection, confirmation and network retry keys.

## Каталог и общий сценарий бронирования

Услуги сгруппированы по `category_name` (например, «Мастер Андрей» и
«Мастер Даниил»). Выбор карточки показывает кнопку «Забронировать онлайн».
Далее используется общий ресторанный `BookingPicker`: календарь и время,
контакты и комментарий, проверка выбранной услуги и суммы, подтверждение.
Количество гостей и предзаказ блюд для индивидуальной услуги не показываются.

`bookable_services.resource_id` связывает услугу с конкретным специалистом
(`venue_tables`). Доступность и создание записи используют одну и ту же
фильтрацию ресурса. Услуги одного мастера делят расписание; разные мастера
могут принимать клиентов одновременно. Пустой ресурс сохраняет прежнее
поведение общего пула. Категория мастера записывается в комментарий брони
для существующей истории. Демо барбершопа содержит двух мастеров.
