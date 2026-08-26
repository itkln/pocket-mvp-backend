package demodata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func seedCatalog(ctx context.Context, tx pgx.Tx, venueID string) error {
	for _, category := range catalogCategories(venueID) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO menu_categories (id, venue_id, name, sort_order, is_active)
			VALUES ($1, $2, $3, $4, $5)`,
			category.ID, venueID, category.Name, category.SortOrder, category.Active,
		); err != nil {
			return fmt.Errorf("seed category %s: %w", category.Name, err)
		}
	}

	for _, item := range catalogItems(venueID) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO menu_items (
				id, venue_id, category_id, name, description, price_minor, currency,
				tax_rate_basis_points, is_available, is_popular, sort_order
			)
			VALUES ($1, $2, $3, $4, $5, $6, 'EUR', 2000, $7, $8, $9)`,
			item.ID, venueID, item.CategoryID, item.Name, item.Description,
			item.PriceMinor, item.Available, item.Popular, item.SortOrder,
		); err != nil {
			return fmt.Errorf("seed menu item %s: %w", item.Name, err)
		}
		if item.ImageURL == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO menu_item_images (
				menu_item_id, storage_key, public_url, alt_text, content_type,
				byte_size, width, height, sort_order
			)
			VALUES ($1, $2, $3, $4, 'image/external', 1, 1200, 800, 0)`,
			item.ID, "demo/"+item.ID, item.ImageURL, item.Name,
		); err != nil {
			return fmt.Errorf("seed image for %s: %w", item.Name, err)
		}
	}
	return nil
}

func catalogCategories(venueID string) []categorySeed {
	if venueID == secondaryVenueID {
		return []categorySeed{
			{ID: "30000000-0000-4000-8000-000000000101", Name: "Кофе", SortOrder: 0, Active: true},
			{ID: "30000000-0000-4000-8000-000000000102", Name: "Выпечка", SortOrder: 1, Active: true},
			{ID: "30000000-0000-4000-8000-000000000103", Name: "Завтраки", SortOrder: 2, Active: true},
		}
	}
	return []categorySeed{
		{ID: "30000000-0000-4000-8000-000000000001", Name: "Завтраки", SortOrder: 0, Active: true},
		{ID: "30000000-0000-4000-8000-000000000002", Name: "Закуски", SortOrder: 1, Active: true},
		{ID: "30000000-0000-4000-8000-000000000003", Name: "Основные блюда", SortOrder: 2, Active: true},
		{ID: "30000000-0000-4000-8000-000000000004", Name: "Десерты", SortOrder: 3, Active: true},
		{ID: "30000000-0000-4000-8000-000000000005", Name: "Напитки", SortOrder: 4, Active: true},
	}
}

func catalogItems(venueID string) []itemSeed {
	if venueID == secondaryVenueID {
		return []itemSeed{
			{ID: "40000000-0000-4000-8000-000000000101", CategoryID: "30000000-0000-4000-8000-000000000101", Name: "Фильтр-кофе", Description: "Эфиопия, натуральная обработка, ноты ягод и какао.", PriceMinor: 390, Available: true, Popular: true, SortOrder: 0, ImageURL: "https://images.unsplash.com/photo-1495474472287-4d71bcdd2085?auto=format&fit=crop&w=900&q=85"},
			{ID: "40000000-0000-4000-8000-000000000102", CategoryID: "30000000-0000-4000-8000-000000000101", Name: "Капучино", Description: "Двойной эспрессо и молоко с шелковистой текстурой.", PriceMinor: 420, Available: true, Popular: true, SortOrder: 1, ImageURL: "https://images.unsplash.com/photo-1572442388796-11668a67e53d?auto=format&fit=crop&w=900&q=85"},
			{ID: "40000000-0000-4000-8000-000000000103", CategoryID: "30000000-0000-4000-8000-000000000102", Name: "Круассан", Description: "Сливочное тесто, выпекаем каждое утро.", PriceMinor: 320, Available: true, Popular: false, SortOrder: 0, ImageURL: "https://images.unsplash.com/photo-1555507036-ab1f4038808a?auto=format&fit=crop&w=900&q=85"},
			{ID: "40000000-0000-4000-8000-000000000104", CategoryID: "30000000-0000-4000-8000-000000000103", Name: "Авокадо-тост", Description: "Авокадо, яйцо пашот, томаты и хлеб на закваске.", PriceMinor: 890, Available: true, Popular: false, SortOrder: 0, ImageURL: "https://images.unsplash.com/photo-1525351484163-7529414344d8?auto=format&fit=crop&w=900&q=85"},
		}
	}
	return []itemSeed{
		{ID: "40000000-0000-4000-8000-000000000001", CategoryID: "30000000-0000-4000-8000-000000000001", Name: "Шакшука", Description: "Яйца, томаты, сладкий перец, фета и зелень.", PriceMinor: 1090, Available: true, Popular: true, SortOrder: 0, ImageURL: "https://images.unsplash.com/photo-1590412200988-a436970781fa?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000002", CategoryID: "30000000-0000-4000-8000-000000000001", Name: "Французский тост", Description: "Бриошь, сезонные ягоды, маскарпоне и кленовый сироп.", PriceMinor: 950, Available: true, Popular: false, SortOrder: 1, ImageURL: "https://images.unsplash.com/photo-1484723091739-30a097e8f929?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000003", CategoryID: "30000000-0000-4000-8000-000000000002", Name: "Буррата с томатами", Description: "Буррата, томаты, базилик и оливковое масло.", PriceMinor: 1290, Available: true, Popular: true, SortOrder: 0, ImageURL: "https://images.unsplash.com/photo-1625944525533-473f1a3d54e7?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000004", CategoryID: "30000000-0000-4000-8000-000000000002", Name: "Тартар из говядины", Description: "Говядина, каперсы, горчица и хрустящий хлеб.", PriceMinor: 1490, Available: true, Popular: false, SortOrder: 1, ImageURL: "https://images.unsplash.com/photo-1544025162-d76694265947?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000005", CategoryID: "30000000-0000-4000-8000-000000000003", Name: "Тальятелле с грибами", Description: "Домашняя паста, лесные грибы, пармезан и трюфель.", PriceMinor: 1790, Available: true, Popular: true, SortOrder: 0, ImageURL: "https://images.unsplash.com/photo-1473093295043-cdd812d0e601?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000006", CategoryID: "30000000-0000-4000-8000-000000000003", Name: "Лосось на гриле", Description: "Лосось, молодой картофель, шпинат и лимонный соус.", PriceMinor: 2190, Available: true, Popular: false, SortOrder: 1, ImageURL: "https://images.unsplash.com/photo-1467003909585-2f8a72700288?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000007", CategoryID: "30000000-0000-4000-8000-000000000003", Name: "Утиная грудка", Description: "Утка, пюре из сельдерея, вишня и красное вино.", PriceMinor: 2390, Available: false, Popular: false, SortOrder: 2, ImageURL: "https://images.unsplash.com/photo-1547592180-85f173990554?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000008", CategoryID: "30000000-0000-4000-8000-000000000004", Name: "Баскский чизкейк", Description: "Кремовый чизкейк с карамельной корочкой.", PriceMinor: 790, Available: true, Popular: true, SortOrder: 0, ImageURL: "https://images.unsplash.com/photo-1578985545062-69928b1d9587?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000009", CategoryID: "30000000-0000-4000-8000-000000000004", Name: "Тирамису", Description: "Маскарпоне, эспрессо и какао.", PriceMinor: 750, Available: true, Popular: false, SortOrder: 1, ImageURL: "https://images.unsplash.com/photo-1571877227200-a0d98ea607e9?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000010", CategoryID: "30000000-0000-4000-8000-000000000005", Name: "Домашний лимонад", Description: "Лимон, мята, бузина и газированная вода.", PriceMinor: 490, Available: true, Popular: true, SortOrder: 0, ImageURL: "https://images.unsplash.com/photo-1523371054106-bbf80586c38c?auto=format&fit=crop&w=900&q=85"},
		{ID: "40000000-0000-4000-8000-000000000011", CategoryID: "30000000-0000-4000-8000-000000000005", Name: "Эспрессо-тоник", Description: "Двойной эспрессо, тоник и апельсин.", PriceMinor: 550, Available: true, Popular: false, SortOrder: 1, ImageURL: "https://images.unsplash.com/photo-1517701604599-bb29b565090c?auto=format&fit=crop&w=900&q=85"},
	}
}
