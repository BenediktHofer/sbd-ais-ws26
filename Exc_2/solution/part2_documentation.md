# Part 2

> ##### Load the generated data into PostgreSQL in a new table called orders. Write the CREATE TABLE yourself (choose sensible types for each column) and use the same \COPY pattern as in section 3.3.

**Columns:**

```
customer_name, product_category, quantity, price_per_unit, order_date, country
```

**Create Statement & COPY:**

```
DROP TABLE IF EXISTS orders;

CREATE TABLE orders (
  id SERIAL PRIMARY KEY,
  customer_name TEXT,
  product_category TEXT,
  quantity INTEGER,
  price_per_unit FLOAT,
  order_date DATE,
  country TEXT
);

\COPY orders(customer_name,product_category,quantity,price_per_unit,order_date,country) FROM '/data/orders_1M.csv' DELIMITER ',' CSV HEADER;

```

> ##### A. Which order has the highest price_per_unit?

```
SELECT * FROM orders WHERE price_per_unit >= (SELECT MAX(price_per_unit) FROM orders);
```

**Result:**

```
   id   | customer_name | product_category | quantity | price_per_unit | order_date | country 
--------+---------------+------------------+----------+----------------+------------+---------
 841292 | Emma Brown    | Automotive       |        3 |           2000 | 2024-10-11 | Italy
```

> ##### B. What are the top 3 product categories with the highest total quantity sold across all orders?

```
SELECT product_category, SUM(quantity) AS total_sold FROM orders GROUP BY product_category ORDER BY total_sold DESC LIMIT 3;
```

**Result:**

```
 product_category | total_sold 
------------------+------------
 Health & Beauty  |     300842
 Electronics      |     300804
 Toys             |     300598
```

> ##### C. What is the total revenue per product category? (Revenue = price_per_unit × quantity)

```
SELECT product_category, SUM(price_per_unit * quantity) AS total_revenue FROM orders GROUP BY product_category;
```

**Result:**

```
 product_category |   total_revenue    
------------------+--------------------
 Automotive       | 306589798.85999954
 Books            | 12731976.040000048
 Electronics      | 241525009.45000046
 Fashion          | 31566368.220000003
 Grocery          | 15268355.660000004
 Health & Beauty  |  46599817.89000005
 Home & Garden    |  78023780.09000033
 Office Supplies  |  38276061.63999991
 Sports           |  61848990.82999989
 Toys             | 23271039.019999914
```

> ##### D. Who are the top 5 customers by total spending?

```
SELECT customer_name, SUM(price_per_unit * quantity) as total_spending, COUNT(customer_name) as total_orders FROM orders GROUP BY customer_name ORDER BY total_spending DESC LIMIT 5;
```

**Result:**

```
 customer_name  |  total_spending   | total_orders 
----------------+-------------------+--------------
 Carol Taylor   | 991179.1799999998 |         1028
 Nina Lopez     |         975444.95 |          980
 Daniel Jackson |         959344.48 |         1033
 Carol Lewis    | 947708.5699999998 |          943
 Daniel Young   | 946030.1400000004 |          973
(5 rows)
```

> ##### E. Look at the spending totals in D — and at how many orders each of those customers has. What do you notice? Open ecommerce/dataset_generator.py and explain why the data looks like this.

We write 1,000,000 rows but have a more limited pool of possible combinations (32 × 33 = 1056) for firstname and lastname, which is why single customers have a lot of orders.

## 2.2

```
SELECT COUNT(*)
FROM people_big p1
JOIN people_big p2
  ON p1.country = p2.country;
```

> ##### On the full 1M-row table this takes more than 10 minutes and slows down the whole database for everyone else. Your job is to find out why before proposing a fix.

Each country produces (people in that country)^2 rows. There are 92 countries. Summed over all countries it is about 11 billion rows.

> ##### Run the self-join (with \timing on) on each of the three tables and fill in:

```
SELECT COUNT(*)
FROM people_50k p1
JOIN people_50k p2
  ON p1.country = p2.country;
```

**Result:**

```
  count   
----------
 27501822
(1 row)

Time: 3385.762 ms (00:03.386)
```
<br><br>

```
SELECT COUNT(*)
FROM people_100k p1
JOIN people_100k p2
  ON p1.country = p2.country;
```

**Result:**

```
   count   
-----------
 109946508
(1 row)

Time: 8170.995 ms (00:08.171)
```

<br><br>

```
SELECT COUNT(*)
FROM people_200k p1
JOIN people_200k p2
  ON p1.country = p2.country;
```

**Result:**

```
   count   
-----------
 439395606
(1 row)

Time: 32227.943 ms (00:32.228)
```

<br>

### Table:

```
| rows in table | join result (COUNT(*)) | time      |
|---------------|------------------------|-----------|
| 50 000        | 27,501,822             | 00:03.386 |
| 100 000       | 109,946,508            | 00:08.171 |
| 200 000       | 439,395,606            | 00:32.228 |	
```

<br>

> ##### When the input doubles, by what factor do the result and the time grow? Use this to predict the result size and the runtime on people_big (1M rows).

When the input doubles, the time grows 4x roughly. 1M is 5x of 200k, so both numbers grow by 5^2 which is 25x. Result Set: 439,395,606 x 25 = 10.98 billion rows.
Time: 32.2s x 25 = 806s = 13.5 minutes

> ##### Step 2 — Does an index help? Create an index on country of people_100k, run ANALYZE people_100k;, and repeat the query. Did the time change? Use EXPLAIN ANALYZE to support your answer. Why does (or doesn't) the index help?

```
CREATE INDEX idx_people_100k_country ON people_100k (country);
```

```
ANALYZE people_100k;
```

```
SELECT COUNT(*)
FROM people_100k p1
JOIN people_100k p2 ON p1.country = p2.country;
```

**Result:**

```
   count   
-----------
 109946508
(1 row)

Time: 7505.904 ms (00:07.506)
```


The time will barely change. The bottleneck is the roughly 110 million output pairs, not finding the rows, index doesn't work here.

> ##### Step 3 — Rewrite it. The query only wants the number of matching pairs, not the pairs themselves. If a country has k people, how many pairs does it contribute to the join? Write a query that computes the same number without a join. Check that it returns exactly the same result as the join on people_100k, then run it on people_big and compare its runtime with your prediction from Step 1.

Each of the k people gets matched with all k people in the same country, themselves included. So one country adds k × k = k² rows

```
SELECT COUNT(*)
FROM people_100k p1
JOIN people_100k p2 ON p1.country = p2.country;
```

**Result:**

```
   count   
-----------
 109946508
(1 row)

Time: 7505.904 ms (00:07.506)
```

<br>

**Improvement:**

```
SELECT SUM(cnt * cnt) AS pairs
FROM (
    SELECT country, COUNT(*) AS cnt
    FROM people_100k
    GROUP BY country
) t;
```

**Result:**

```
   pairs   
-----------
 109946508
(1 row)

Time: 23.619 ms
```

Same result amount!

**Running on people_big:**

```
SELECT SUM(cnt * cnt) AS pairs_big
FROM (
    SELECT country, COUNT(*) AS cnt
    FROM people_big
    GROUP BY country
) t;
```

```
  pairs_big  
-------------
 10983941260
(1 row)

Time: 885.708 ms
```

> ##### Step 4 — Discussion (submit in writing). Considering scalability and efficiency, which approaches and/or optimizations can be applied to improve this kind of query in a real system? Discuss at least: 
> - what the rewrite in Step 3 tells you about adding more hardware or an index;
> - what you would do if the business actually needed the pairs themselves (not just their count) — would a bigger machine or a cluster help, and how much?
> - the limits of an OLTP database for this workload, especially in a large-scale cloud environment.

- We can see that adding more hardware or an index is often not the problem and that we should prioritize query efficiency first of all.
-  Speedup is at most linear: 2× the cores gives at most 2× faster, and usually less because memory and disk become the limit. From about 13.5 minutes just to count, with 4× the hardware you'd get around 3 to 4 minutes.Writing 0.2 to 1 TB of output adds disk time on top. It doesn't change the growth. At 10M rows the work is 100× larger (about 1.1 trillion pairs), which outgrows any machine. The problem splits well by country. Each country's pairs are independent, so you can partition by country and give each node some countries. Splitting big countries into sub-chunks helps. 10 nodes would take roughly 1.5 to 2 minutes of compute, but you still write 176 GB to 1.1 TB in total.
- An OLTP database like Postgres is built for many small, quick transactions, so a huge query like this runs on one machine and slows down everyone else using the database. A bigger cloud machine only makes it a little faster, and the problem stays: more data means quadratically more pairs, so the real fix is to rewrite the query (like Step 3), not to buy more hardware.