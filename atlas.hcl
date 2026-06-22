// Define the "local" environment which represents our local postgres instance
env "local" {
  src = "ent://ent/schema"
  dev = "postgres://postgres:postgres@localhost:5432/ishos_dev?sslmode=disable"
  
  migration {
    dir = "file://ent/migrate/migrations"
    format = golang-migrate
  }
}

// Define the "prod" environment for production database
env "prod" {
  src = "ent://ent/schema"
  dev = "postgres://postgres:postgres@localhost:5432/ishos_dev?sslmode=disable"
  
  migration {
    dir = "file://ent/migrate/migrations"
    format = golang-migrate
  }
}
