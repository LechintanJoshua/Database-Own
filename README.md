# Database-Own

* **Acest repository conține implementarea unui motor de baze de date.**
* Proiectul este împărțit pe două branch-uri principale:

1. Branch-ul **main** (Versiune proiect final)
* Aici se afla implementarea de baza. Acest branch reprezinta partea principala a proiectului care va urma a fi dezvoltata in continuare
* Roadmap viitor pentru main:

* Suport pentru concurență (Concurrency)

* Implementarea de indecsi secundari

* Dezvoltarea unui mini-parser SQL

2. Branch-ul **sda-project** (Versiunea pentru facultate)
* Acest branch contine versiunea proiectului adaptata si extinsa pentru cerintele de la facultate. Dezvoltarea pe acest branch este stabila si include functionalitati extra fata de partea curenta din main:

* Interfata CLI: Implementata folosind libraria Cobra pentru o interactiune usoara cu motorul.

* Managementul Memoriei: Implementarea unui LRU Page Cache custom pentru optimizare si utilizare hashtable.

* Strat Relational: Operatiuni complete (Insert, Get, Update, Delete, Range Queries si List) construite direct peste structura interna B+Tree si sistemul de MMap.

* Pentru a vizualiza implementarea predată la facultate, te rog să schimbi pe branch-ul `sda-project`.