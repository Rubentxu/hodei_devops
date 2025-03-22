db = db.getSiblingDB('hodei');

// Crear colecciones
db.createCollection('resourcepools');
db.createCollection('tasks');
db.createCollection('workerdefinitions');
db.createCollection('taskexecutions');
db.createCollection('users');

// Añadir un usuario administrador para pruebas
db.users.insertOne({
  username: "admin",
  password: "$2a$12$1InE3Tq5Y0r4goPYdpjRa.YHPvW84XjZYHwUJQR1WhsQ/FkHuSIde", // "admin123" encriptado con bcrypt
  roles: ["admin"]
});