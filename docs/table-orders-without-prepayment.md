# V-TBL-03: table orders without prepayment

Guests use the existing QR menu, cart, notes and server-side price/availability
checks. Creating an order does not require a payment. New orders enter the
Vendor/kitchen queue with the table and current bill ID. Each additional order
increases the same open visit; kitchen transitions through completed do not
settle it. Payment and staff settlement remain separate tasks.

The client persists the exact submission before sending. Until the server
confirms acceptance or definitively rejects it, cart edits and repeat-order
merging are blocked. Retrying, including after a reload, reuses the original
payload and idempotency key. Lookup can recover a committed order after a lost
response without resubmitting. Only confirmed acceptance clears the cart.
Definitive price, availability, validation or bill-lock rejections keep its
contents and unlock editing. Browsers that disallow session storage retain
in-memory retries only.

Validation: frontend table/cart, booking, preorder and notification tests;
TestTableOrderWithoutPrepaymentIntegration plus existing table QR, visit,
order queue/policy and HTTP tests in isolated PostgreSQL. The added integration
scenario checks two unpaid orders, one visit, queue metadata, repeated requests,
all kitchen status transitions, no payment records and no visit closure.
