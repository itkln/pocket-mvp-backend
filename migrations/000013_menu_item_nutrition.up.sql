ALTER TABLE menu_items
    ADD COLUMN portion_weight_g integer CHECK (portion_weight_g > 0),
    ADD COLUMN energy_kcal numeric(8,2) CHECK (energy_kcal >= 0),
    ADD COLUMN protein_g numeric(5,2) CHECK (protein_g BETWEEN 0 AND 100),
    ADD COLUMN fat_g numeric(5,2) CHECK (fat_g BETWEEN 0 AND 100),
    ADD COLUMN carbs_g numeric(5,2) CHECK (carbs_g BETWEEN 0 AND 100);
