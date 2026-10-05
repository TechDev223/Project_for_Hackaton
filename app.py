from flask import Flask, render_template
import json

app = Flask(__name__)

# Имитация данных (в реальности это запрос к БД)
def get_inventory_data():
    return [
        {"id": 1, "name": "Тормозные колодки", "stock": 100, "incoming": 100, "min_stock": 200},
        {"id": 2, "name": "ЭБУ (Чип)", "stock": 10, "incoming": 0, "min_stock": 50},
        {"id": 3, "name": "Шины R17", "stock": 800, "incoming": 2000, "min_stock": 300}
    ]

@app.route('/')
def dashboard():
    inventory = get_inventory_data()
    
    # Добавляем статус прямо в Python перед передачей в шаблон
    for item in inventory:
        total = item['stock'] + item['incoming']
        if total < item['min_stock']:
            item['status'] = 'critical'
            item['status_text'] = 'КРИТИЧЕСКИ МАЛО'
        elif total <= item['min_stock']:
            item['status'] = 'warning'
            item['status_text'] = 'ВНИМАНИЕ'
        else:
            item['status'] = 'ok'
            item['status_text'] = 'НОРМА'
            
    return render_template('index.html', inventory=inventory)

if __name__ == '__main__':
    app.run(debug=True)
