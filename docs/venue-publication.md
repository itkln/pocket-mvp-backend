# Venue publication, profile and opening hours (V-05)

What the owner sets in Vendor (Заведение) is what guests see and can do in Dish.

## Status

| Status   | Dish catalog and venue page | New bookings | Table QR                                   |
|----------|-----------------------------|--------------|--------------------------------------------|
| `draft`  | hidden                      | refused      | invalid (`qr_invalid`)                     |
| `active` | listed                      | accepted     | ordering per the "Заказы в зале" switch    |
| `paused` | hidden                      | refused      | opens (bill, history) but `ordering_enabled: false`; new orders get `bill_locked` |
| `closed` | hidden                      | refused      | invalid                                    |

Existing bookings are not cancelled by a pause; seated guests keep their bill.

## Profile in Dish

`GET /public/venues/{id}` carries the name, address, `phone`, `email`, `timezone` and
`opening_hours` from the Vendor profile.

## Opening hours

Stored in `venues.settings.opening_hours`:

```json
{"mon": {"open": "09:00", "close": "22:00"}, "tue": null, "...": "...", "sun": null}
```

All seven days are required; `null` is a closed day; `close` is after `open` on the same day
(`"24:00"` = midnight). Invalid hours or an unknown timezone are refused with `422`. Until the
venue sets them, every day is 09:00–22:00. Booking slots (availability) start at opening time and
end by closing time; a closed day offers none.

## Switches

Vendor and the backend use the same keys and defaults (all on for a new venue):

| Vendor switch                | Key                | Effect                                                              |
|------------------------------|--------------------|---------------------------------------------------------------------|
| Онлайн-бронирование          | `schedule_enabled` | off: no slots, bookings refused; Dish shows booking as unavailable (`booking_enabled: false`) |
| Разрешить бронирование сегодня | `same_day_booking` | off: no slots for today                                           |
| Заказы в зале                | `dine_in`          | off: the table QR opens with `ordering_enabled: false`              |
| Предзаказ при бронировании   | `preorder_enabled` | off: bookings with dishes refused (`item_unavailable`); Dish hides preorder |

The first switch used to read "Заведение открыто по расписанию — принимать брони только в
рабочие часы", but it has always switched online booking on and off; it was renamed. Migration `000028` moved
the old Vendor key `preorder` and the demo key `dine_in_enabled` to these.
