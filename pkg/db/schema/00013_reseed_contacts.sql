-- +goose Up
ALTER TABLE contacts ADD COLUMN linkedin TEXT NOT NULL DEFAULT '';
ALTER TABLE contacts ADD COLUMN location TEXT NOT NULL DEFAULT '';
ALTER TABLE contacts ADD COLUMN phone TEXT NOT NULL DEFAULT '';
ALTER TABLE contacts ADD COLUMN status TEXT NOT NULL DEFAULT '';
ALTER TABLE contacts ADD COLUMN twitter TEXT NOT NULL DEFAULT '';

DELETE FROM emails WHERE contact_id BETWEEN 1 AND 40;
DELETE FROM contacts WHERE id BETWEEN 1 AND 40;

INSERT INTO contacts (id, company, created_at, job_title, linkedin, location, name, phone, status, twitter, updated_at) VALUES
  (1, 'Stark Industries', '2026-06-01T12:00:00Z', 'Chief Executive Officer', 'in/tony-stark', 'Malibu, CA', 'Tony Stark', '+1 (555) 010-0001', 'Customer', '@ironman', '2026-06-01T12:00:00Z'),
  (2, 'Wayne Enterprises', '2026-06-01T12:00:00Z', 'Chief Executive Officer', 'in/bruce-wayne', 'Gotham City', 'Bruce Wayne', '+1 (555) 010-0002', 'Customer', '@thedarkknight', '2026-06-01T12:00:00Z'),
  (3, 'Weyland-Yutani', '2026-06-01T12:00:00Z', 'Warrant Officer', 'in/ellen-ripley', 'LV-426', 'Ellen Ripley', '+1 (555) 010-0003', 'Prospect', '@nostromo', '2026-06-01T12:00:00Z'),
  (4, 'Cyberdyne Resistance', '2026-06-01T12:00:00Z', 'Field Operative', 'in/sarah-connor', 'Los Angeles, CA', 'Sarah Connor', '+1 (555) 010-0004', 'Lead', '@nofate', '2026-06-01T12:00:00Z'),
  (5, 'Gray Matter', '2026-06-01T12:00:00Z', 'Head of Chemistry', 'in/walter-white', 'Albuquerque, NM', 'Walter White', '+1 (555) 010-0005', 'Churned', '@heisenberg', '2026-06-01T12:00:00Z'),
  (6, 'Dunder Mifflin', '2026-06-01T12:00:00Z', 'Regional Manager', 'in/michael-scott', 'Scranton, PA', 'Michael Scott', '+1 (555) 010-0006', 'Customer', '@worldsbestboss', '2026-06-01T12:00:00Z'),
  (7, 'Pawnee Parks Dept', '2026-06-01T12:00:00Z', 'Deputy Director', 'in/leslie-knope', 'Pawnee, IN', 'Leslie Knope', '+1 (555) 010-0007', 'Champion', '@knope2012', '2026-06-01T12:00:00Z'),
  (8, 'Pawnee Parks Dept', '2026-06-01T12:00:00Z', 'Director', 'in/ron-swanson', 'Pawnee, IN', 'Ron Swanson', '+1 (555) 010-0008', 'Customer', '@baconandeggs', '2026-06-01T12:00:00Z'),
  (9, 'Black Pearl', '2026-06-01T12:00:00Z', 'Captain', 'in/jack-sparrow', 'Tortuga', 'Jack Sparrow', '+1 (555) 010-0009', 'Lead', '@captainjack', '2026-06-01T12:00:00Z'),
  (10, 'Ministry of Magic', '2026-06-01T12:00:00Z', 'Minister for Magic', 'in/hermione-granger', 'London, UK', 'Hermione Granger', '+1 (555) 010-0010', 'Customer', '@hgranger', '2026-06-01T12:00:00Z'),
  (11, 'Ministry of Magic', '2026-06-01T12:00:00Z', 'Head Auror', 'in/harry-potter', 'London, UK', 'Harry Potter', '+1 (555) 010-0011', 'Customer', '@theboywholived', '2026-06-01T12:00:00Z'),
  (12, 'Bag End', '2026-06-01T12:00:00Z', 'Ring-bearer', 'in/frodo-baggins', 'The Shire', 'Frodo Baggins', '+1 (555) 010-0012', 'Prospect', '@ringbearer', '2026-06-01T12:00:00Z'),
  (13, 'Istari Order', '2026-06-01T12:00:00Z', 'Wizard', 'in/gandalf-grey', 'Middle-earth', 'Gandalf Grey', '+1 (555) 010-0013', 'Partner', '@youshallnotpass', '2026-06-01T12:00:00Z'),
  (14, 'District 12', '2026-06-01T12:00:00Z', 'Victor', 'in/katniss-everdeen', 'Panem', 'Katniss Everdeen', '+1 (555) 010-0014', 'Lead', '@mockingjay', '2026-06-01T12:00:00Z'),
  (15, 'Tyrell Corp', '2026-06-01T12:00:00Z', 'Blade Runner', 'in/rick-deckard', 'Los Angeles, CA', 'Rick Deckard', '+1 (555) 010-0015', 'Prospect', '@bladerunner', '2026-06-01T12:00:00Z'),
  (16, '221B Consulting', '2026-06-01T12:00:00Z', 'Consulting Detective', 'in/sherlock-holmes', 'London, UK', 'Sherlock Holmes', '+1 (555) 010-0016', 'Customer', '@thegame', '2026-06-01T12:00:00Z'),
  (17, 'FBI X-Files', '2026-06-01T12:00:00Z', 'Special Agent', 'in/dana-scully', 'Washington, DC', 'Dana Scully', '+1 (555) 010-0017', 'Customer', '@agentscully', '2026-06-01T12:00:00Z'),
  (18, 'FBI X-Files', '2026-06-01T12:00:00Z', 'Special Agent', 'in/fox-mulder', 'Washington, DC', 'Fox Mulder', '+1 (555) 010-0018', 'Customer', '@ibelieve', '2026-06-01T12:00:00Z'),
  (19, 'House Lannister', '2026-06-01T12:00:00Z', 'Hand of the Queen', 'in/tyrion-lannister', 'King''s Landing', 'Tyrion Lannister', '+1 (555) 010-0019', 'Partner', '@impster', '2026-06-01T12:00:00Z'),
  (20, 'House Targaryen', '2026-06-01T12:00:00Z', 'Queen', 'in/daenerys-targaryen', 'Meereen', 'Daenerys Targaryen', '+1 (555) 010-0020', 'Lead', '@motherofdragons', '2026-06-01T12:00:00Z'),
  (21, 'Night''s Watch', '2026-06-01T12:00:00Z', 'Lord Commander', 'in/jon-snow', 'The Wall', 'Jon Snow', '+1 (555) 010-0021', 'Prospect', '@knownothing', '2026-06-01T12:00:00Z'),
  (22, 'House Stark', '2026-06-01T12:00:00Z', 'Faceless Operative', 'in/arya-stark', 'Winterfell', 'Arya Stark', '+1 (555) 010-0022', 'Lead', '@nolonger', '2026-06-01T12:00:00Z'),
  (23, 'Zion', '2026-06-01T12:00:00Z', 'Operator', 'in/neo-anderson', 'The Matrix', 'Neo Anderson', '+1 (555) 010-0023', 'Champion', '@theone', '2026-06-01T12:00:00Z'),
  (24, 'Zion', '2026-06-01T12:00:00Z', 'Captain', 'in/trinity-moss', 'The Matrix', 'Trinity Moss', '+1 (555) 010-0024', 'Customer', '@followthewhiterabbit', '2026-06-01T12:00:00Z'),
  (25, 'Hill Valley High', '2026-06-01T12:00:00Z', 'Student', 'in/marty-mcfly', 'Hill Valley, CA', 'Marty McFly', '+1 (555) 010-0025', 'Prospect', '@88mph', '2026-06-01T12:00:00Z'),
  (26, 'Brown Labs', '2026-06-01T12:00:00Z', 'Inventor', 'in/emmett-brown', 'Hill Valley, CA', 'Emmett Brown', '+1 (555) 010-0026', 'Partner', '@greatscott', '2026-06-01T12:00:00Z'),
  (27, 'Ghostbusters', '2026-06-01T12:00:00Z', 'Founder', 'in/peter-venkman', 'New York, NY', 'Peter Venkman', '+1 (555) 010-0027', 'Customer', '@whoyougonnacall', '2026-06-01T12:00:00Z'),
  (28, 'Bueller & Co', '2026-06-01T12:00:00Z', 'Founder', 'in/ferris-bueller', 'Chicago, IL', 'Ferris Bueller', '+1 (555) 010-0028', 'Lead', '@dayoff', '2026-06-01T12:00:00Z'),
  (29, 'Woods Law', '2026-06-01T12:00:00Z', 'Attorney', 'in/elle-woods', 'Cambridge, MA', 'Elle Woods', '+1 (555) 010-0029', 'Customer', '@bendandsnap', '2026-06-01T12:00:00Z'),
  (30, 'Bubba Gump Shrimp', '2026-06-01T12:00:00Z', 'Founder', 'in/forrest-gump', 'Greenbow, AL', 'Forrest Gump', '+1 (555) 010-0030', 'Customer', '@runforrest', '2026-06-01T12:00:00Z'),
  (31, 'Genco Olive Oil', '2026-06-01T12:00:00Z', 'Founder', 'in/vito-corleone', 'New York, NY', 'Vito Corleone', '+1 (555) 010-0031', 'Partner', '@thedon', '2026-06-01T12:00:00Z'),
  (32, 'FBI BSU', '2026-06-01T12:00:00Z', 'Special Agent', 'in/clarice-starling', 'Quantico, VA', 'Clarice Starling', '+1 (555) 010-0032', 'Prospect', '@quidproquo', '2026-06-01T12:00:00Z'),
  (33, 'MI6', '2026-06-01T12:00:00Z', 'Agent', 'in/james-bond', 'London, UK', 'James Bond', '+1 (555) 010-0033', 'Customer', '@007', '2026-06-01T12:00:00Z'),
  (34, 'Croft Holdings', '2026-06-01T12:00:00Z', 'Archaeologist', 'in/lara-croft', 'Surrey, UK', 'Lara Croft', '+1 (555) 010-0034', 'Lead', '@tombraider', '2026-06-01T12:00:00Z'),
  (35, 'IMF', '2026-06-01T12:00:00Z', 'Field Agent', 'in/ethan-hunt', 'Washington, DC', 'Ethan Hunt', '+1 (555) 010-0035', 'Prospect', '@missionpossible', '2026-06-01T12:00:00Z'),
  (36, 'Deadly Vipers', '2026-06-01T12:00:00Z', 'Specialist', 'in/beatrix-kiddo', 'El Paso, TX', 'Beatrix Kiddo', '+1 (555) 010-0036', 'Churned', '@thebride', '2026-06-01T12:00:00Z'),
  (37, 'Marsellus Inc', '2026-06-01T12:00:00Z', 'Associate', 'in/jules-winnfield', 'Los Angeles, CA', 'Jules Winnfield', '+1 (555) 010-0037', 'Lead', '@ezekiel2517', '2026-06-01T12:00:00Z'),
  (38, 'The Citadel', '2026-06-01T12:00:00Z', 'Imperator', 'in/furiosa-jabassa', 'The Wasteland', 'Furiosa Jabassa', '+1 (555) 010-0038', 'Prospect', '@warrig', '2026-06-01T12:00:00Z'),
  (39, 'Toretto Garage', '2026-06-01T12:00:00Z', 'Owner', 'in/dominic-toretto', 'Los Angeles, CA', 'Dominic Toretto', '+1 (555) 010-0039', 'Customer', '@family', '2026-06-01T12:00:00Z'),
  (40, 'Nevermore Academy', '2026-06-01T12:00:00Z', 'Student', 'in/wednesday-addams', 'Jericho, VT', 'Wednesday Addams', '+1 (555) 010-0040', 'Lead', '@thething', '2026-06-01T12:00:00Z');

INSERT INTO emails (contact_id, created_at, email, is_primary, updated_at) VALUES
  (1, '2026-06-01T12:00:00Z', 'tony@stark.com', 1, '2026-06-01T12:00:00Z'),
  (1, '2026-06-01T12:00:00Z', 'tony.stark@avengers.org', 0, '2026-06-01T12:00:00Z'),
  (2, '2026-06-01T12:00:00Z', 'bruce@wayne.com', 1, '2026-06-01T12:00:00Z'),
  (3, '2026-06-01T12:00:00Z', 'ripley@weyland.corp', 1, '2026-06-01T12:00:00Z'),
  (4, '2026-06-01T12:00:00Z', 'sarah@resistance.net', 1, '2026-06-01T12:00:00Z'),
  (5, '2026-06-01T12:00:00Z', 'walter@graymatter.com', 1, '2026-06-01T12:00:00Z'),
  (5, '2026-06-01T12:00:00Z', 'ww@lospollos.com', 0, '2026-06-01T12:00:00Z'),
  (6, '2026-06-01T12:00:00Z', 'michael@dundermifflin.com', 1, '2026-06-01T12:00:00Z'),
  (7, '2026-06-01T12:00:00Z', 'leslie@pawnee.gov', 1, '2026-06-01T12:00:00Z'),
  (8, '2026-06-01T12:00:00Z', 'ron@pawnee.gov', 1, '2026-06-01T12:00:00Z'),
  (9, '2026-06-01T12:00:00Z', 'jack@blackpearl.sea', 1, '2026-06-01T12:00:00Z'),
  (10, '2026-06-01T12:00:00Z', 'hermione@ministry.magic', 1, '2026-06-01T12:00:00Z'),
  (11, '2026-06-01T12:00:00Z', 'harry@ministry.magic', 1, '2026-06-01T12:00:00Z'),
  (12, '2026-06-01T12:00:00Z', 'frodo@bagend.shire', 1, '2026-06-01T12:00:00Z'),
  (13, '2026-06-01T12:00:00Z', 'gandalf@istari.me', 1, '2026-06-01T12:00:00Z'),
  (14, '2026-06-01T12:00:00Z', 'katniss@district12.panem', 1, '2026-06-01T12:00:00Z'),
  (15, '2026-06-01T12:00:00Z', 'deckard@tyrell.corp', 1, '2026-06-01T12:00:00Z'),
  (16, '2026-06-01T12:00:00Z', 'sherlock@221b.co.uk', 1, '2026-06-01T12:00:00Z'),
  (17, '2026-06-01T12:00:00Z', 'scully@fbi.gov', 1, '2026-06-01T12:00:00Z'),
  (18, '2026-06-01T12:00:00Z', 'mulder@fbi.gov', 1, '2026-06-01T12:00:00Z'),
  (19, '2026-06-01T12:00:00Z', 'tyrion@lannister.house', 1, '2026-06-01T12:00:00Z'),
  (20, '2026-06-01T12:00:00Z', 'dany@targaryen.house', 1, '2026-06-01T12:00:00Z'),
  (21, '2026-06-01T12:00:00Z', 'jon@nightswatch.org', 1, '2026-06-01T12:00:00Z'),
  (22, '2026-06-01T12:00:00Z', 'arya@stark.house', 1, '2026-06-01T12:00:00Z'),
  (23, '2026-06-01T12:00:00Z', 'neo@zion.net', 1, '2026-06-01T12:00:00Z'),
  (24, '2026-06-01T12:00:00Z', 'trinity@zion.net', 1, '2026-06-01T12:00:00Z'),
  (25, '2026-06-01T12:00:00Z', 'marty@hillvalley.edu', 1, '2026-06-01T12:00:00Z'),
  (26, '2026-06-01T12:00:00Z', 'doc@brownlabs.com', 1, '2026-06-01T12:00:00Z'),
  (27, '2026-06-01T12:00:00Z', 'venkman@ghostbusters.com', 1, '2026-06-01T12:00:00Z'),
  (28, '2026-06-01T12:00:00Z', 'ferris@bueller.co', 1, '2026-06-01T12:00:00Z'),
  (29, '2026-06-01T12:00:00Z', 'elle@woodslaw.com', 1, '2026-06-01T12:00:00Z'),
  (30, '2026-06-01T12:00:00Z', 'forrest@bubbagump.com', 1, '2026-06-01T12:00:00Z'),
  (31, '2026-06-01T12:00:00Z', 'vito@genco.com', 1, '2026-06-01T12:00:00Z'),
  (32, '2026-06-01T12:00:00Z', 'starling@fbi.gov', 1, '2026-06-01T12:00:00Z'),
  (33, '2026-06-01T12:00:00Z', 'bond@mi6.gov.uk', 1, '2026-06-01T12:00:00Z'),
  (34, '2026-06-01T12:00:00Z', 'lara@croftholdings.com', 1, '2026-06-01T12:00:00Z'),
  (35, '2026-06-01T12:00:00Z', 'hunt@imf.gov', 1, '2026-06-01T12:00:00Z'),
  (36, '2026-06-01T12:00:00Z', 'beatrix@vipers.org', 1, '2026-06-01T12:00:00Z'),
  (37, '2026-06-01T12:00:00Z', 'jules@marsellus.com', 1, '2026-06-01T12:00:00Z'),
  (38, '2026-06-01T12:00:00Z', 'furiosa@citadel.war', 1, '2026-06-01T12:00:00Z'),
  (39, '2026-06-01T12:00:00Z', 'dom@torettogarage.com', 1, '2026-06-01T12:00:00Z'),
  (40, '2026-06-01T12:00:00Z', 'wednesday@nevermore.edu', 1, '2026-06-01T12:00:00Z');

-- +goose Down
DELETE FROM emails WHERE contact_id BETWEEN 1 AND 40;
DELETE FROM contacts WHERE id BETWEEN 1 AND 40;
ALTER TABLE contacts DROP COLUMN twitter;
ALTER TABLE contacts DROP COLUMN status;
ALTER TABLE contacts DROP COLUMN phone;
ALTER TABLE contacts DROP COLUMN location;
ALTER TABLE contacts DROP COLUMN linkedin;
