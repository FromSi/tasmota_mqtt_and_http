# tasmota_mqtt_and_http
Ресерч по контролю перепрошитого SONOFF Basic R2 на Tasmota. Нужные ссылки по взаимодействию http://localhost:8080

## Данные для подключения
* Host - <PC_IP_FROM_WIFI>
* Port - 1883
* Topic - main

## Слежка за подключением устройства
Go приложение читает сообщения о подключении

```bash
# имитация запроса реле
$ mosquitto_pub -h localhost -t tele/main/LWT -m "Online"
$ mosquitto_pub -h localhost -t tele/main/LWT -m "Offline"
```

## Включить реле
Включить можно будет через запрос GET http://localhost:8080/power/on

```bash
# имитация чтения реле
$ mosquitto_sub -h localhost -t cmnd/main/Power -v
cmnd/main/Power ON
cmnd/main/Power ON
```

## Выключить реле
Выключить можно будет через запрос GET http://localhost:8080/power/off

```bash
# имитация чтения реле
$ mosquitto_sub -h localhost -t cmnd/main/Power -v
cmnd/main/Power OFF
cmnd/main/Power OFF
```

## Переключить реле
Переключить можно будет через запрос GET http://localhost:8080/power/toggle

```bash
# имитация чтения реле
$ mosquitto_sub -h localhost -t cmnd/main/Power -v
cmnd/main/Power TOGGLE
cmnd/main/Power TOGGLE
```

## Получить статистику реле
Статистику можно получить через запрос GET http://localhost:8080/status

```bash
# имитация чтения реле, перед ответом
$ mosquitto_sub -h localhost -t cmnd/main/Status0 -v
cmnd/main/Status0 (null)
cmnd/main/Status0 (null)
```

Будет ожидание. Либо получим timeout, либо ответ в виде JSON 

```bash
# имитация чтения реле, перед ответом
$ mosquitto_pub -h localhost -t stat/main/STATUS0 -m "{\"message\": \"Hello\"}"
```

## Отключение физической кнопки на реле
Чтобы отключить физическую кнопку, нужно сделать запрос

```bash
# запрос на отключение или включение физического взаимодействия реле
$ mosquitto_pub -h localhost -t cmnd/main/SetOption73 -m "1"
$ mosquitto_pub -h localhost -t cmnd/main/SetOption73 -m "0"
```

