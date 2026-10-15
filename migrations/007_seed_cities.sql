-- ===========================================
-- Seed key cities for route planning and search
-- ===========================================

INSERT INTO cities (name, state)
VALUES
    ('Mumbai', 'Maharashtra'),
    ('Pune', 'Maharashtra'),
    ('Bengaluru', 'Karnataka'),
    ('Chennai', 'Tamil Nadu'),
    ('Hyderabad', 'Telangana'),
    ('Delhi', 'Delhi'),
    ('Goa', 'Goa'),
    ('Kochi', 'Kerala'),
    ('Coimbatore', 'Tamil Nadu'),
    ('Ahmedabad', 'Gujarat'),
    ('Jaipur', 'Rajasthan'),
    ('Kolkata', 'West Bengal'),
    ('Vijayawada', 'Andhra Pradesh'),
    ('Mangaluru', 'Karnataka'),
    ('Madurai', 'Tamil Nadu')
ON CONFLICT (name) DO NOTHING;
