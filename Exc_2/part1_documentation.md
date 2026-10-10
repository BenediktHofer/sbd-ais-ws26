# Exercise 1

> Think about it: does LIMIT 10 make queries (a) and (b) cheaper? Compare with and without it using EXPLAIN ANALYZE.

## (5a):

### With Limit 10:

```
EXPLAIN ANALYZE SELECT department, AVG(salary)
FROM people_big
GROUP BY department
LIMIT 10;
```

**Result:**

```
 Limit  (cost=17189.60..17192.64 rows=10 width=43) (actual time=249.444..252.024 rows=10.00 loops=1)
   Buffers: shared hit=9952
   ->  Finalize GroupAggregate  (cost=17189.60..17215.44 rows=85 width=43) (actual time=249.433..252.000 rows=10.00 loops=1)
         Group Key: department
         Buffers: shared hit=9952
         ->  Gather Merge  (cost=17189.60..17213.36 rows=204 width=43) (actual time=249.377..251.936 rows=31.00 loops=1)
               Workers Planned: 2
               Workers Launched: 2
               Buffers: shared hit=9952
               ->  Sort  (cost=16189.58..16189.79 rows=85 width=43) (actual time=244.952..244.957 rows=60.33 loops=3)
                     Sort Key: department
                     Sort Method: quicksort  Memory: 30kB
                     Buffers: shared hit=9952
                     Worker 0:  Sort Method: quicksort  Memory: 30kB
                     Worker 1:  Sort Method: quicksort  Memory: 30kB
                     ->  Partial HashAggregate  (cost=16186.00..16186.85 rows=85 width=43) (actual time=244.785..244.793 rows=85.00 loops=3)
                           Group Key: department
                           Batches: 1  Memory Usage: 32kB
                           Buffers: shared hit=9936
                           Worker 0:  Batches: 1  Memory Usage: 32kB
                           Worker 1:  Batches: 1  Memory Usage: 32kB
                           ->  Parallel Seq Scan on people_big  (cost=0.00..14102.67 rows=416667 width=15) (actual time=0.062..105.393 rows=333333.33 loops=3)
                                 Buffers: shared hit=9936
 Planning Time: 0.582 ms
 Execution Time: 252.288 ms
(25 rows)
```

### Without Limit 10:

**Result:**

```
 Finalize GroupAggregate  (cost=17189.60..17215.44 rows=85 width=43) (actual time=272.636..274.910 rows=85.00 loops=1)
   Group Key: department
   Buffers: shared hit=9952
   ->  Gather Merge  (cost=17189.60..17213.36 rows=204 width=43) (actual time=272.610..274.824 rows=255.00 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         Buffers: shared hit=9952
         ->  Sort  (cost=16189.58..16189.79 rows=85 width=43) (actual time=263.331..263.337 rows=85.00 loops=3)
               Sort Key: department
               Sort Method: quicksort  Memory: 30kB
               Buffers: shared hit=9952
               Worker 0:  Sort Method: quicksort  Memory: 30kB
               Worker 1:  Sort Method: quicksort  Memory: 30kB
               ->  Partial HashAggregate  (cost=16186.00..16186.85 rows=85 width=43) (actual time=263.155..263.164 rows=85.00 loops=3)
                     Group Key: department
                     Batches: 1  Memory Usage: 32kB
                     Buffers: shared hit=9936
                     Worker 0:  Batches: 1  Memory Usage: 32kB
                     Worker 1:  Batches: 1  Memory Usage: 32kB
                     ->  Parallel Seq Scan on people_big  (cost=0.00..14102.67 rows=416667 width=15) (actual time=0.081..87.616 rows=333333.33 loops=3)
                           Buffers: shared hit=9936
 Planning Time: 0.977 ms
 Execution Time: 275.461 ms
(23 rows)
:
```

## (5b):

### With Limit 10:

```
EXPLAIN ANALYZE SELECT country, AVG(avg_salary)
FROM (
  SELECT country, department, AVG(salary) AS avg_salary
  FROM people_big
  GROUP BY country, department
) sub
GROUP BY country
LIMIT 10;
```

**Result:**

```
 Limit  (cost=18811.57..18938.78 rows=10 width=40) (actual time=447.847..452.189 rows=10.0
0 loops=1)
   Buffers: shared hit=9952
   ->  GroupAggregate  (cost=18811.57..21355.72 rows=200 width=40) (actual time=447.835..4
52.175 rows=10.00 loops=1)
         Group Key: people_big.country
         Buffers: shared hit=9952
         ->  Finalize GroupAggregate  (cost=18811.57..21235.92 rows=7820 width=51) (actual
 time=447.673..452.063 rows=851.00 loops=1)
               Group Key: people_big.country, people_big.department
               Buffers: shared hit=9952
               ->  Gather Merge  (cost=18811.57..20997.41 rows=18768 width=51) (actual time=447.641..451.463 rows=2554.00 loops=1)
                     Workers Planned: 2
                     Workers Launched: 2
                     Buffers: shared hit=9952
                     ->  Sort  (cost=17811.55..17831.10 rows=7820 width=51) (actual time=418.679..418.772 rows=1320.00 loops=3)
                           Sort Key: people_big.country, people_big.department
                           Sort Method: quicksort  Memory: 776kB
                           Buffers: shared hit=9952
                           Worker 0:  Sort Method: quicksort  Memory: 776kB
                           Worker 1:  Sort Method: quicksort  Memory: 776kB
                           ->  Partial HashAggregate  (cost=17227.67..17305.87 rows=7820 width=51) (actual time=363.216..364.123 rows=7820.00 loops=3)
                                 Group Key: people_big.country, people_big.department
                                 Batches: 1  Memory Usage: 1553kB
                                 Buffers: shared hit=9936
                                 Worker 0:  Batches: 1  Memory Usage: 1553kB
                                 Worker 1:  Batches: 1  Memory Usage: 1553kB
                                 ->  Parallel Seq Scan on people_big  (cost=0.00..14102.67 rows=416667 width=23) (actual time=0.066..106.793 rows=333333.33 loops=3)
                                       Buffers: shared hit=9936
 Planning Time: 0.914 ms
 Execution Time: 452.822 ms
(28 rows)
```

### Without Limit 10:

**Result:**

```
HashAggregate  (cost=20538.48..20540.98 rows=200 width=40) (actual time=442.392..442.470 rows=92.00 loops=1)
   Group Key: people_big.country
   Batches: 1  Memory Usage: 56kB
   Buffers: shared hit=9936
   ->  Finalize HashAggregate  (cost=20323.43..20421.18 rows=7820 width=51) (actual time=439.678..441.345 rows=7820.00 loops=1)
         Group Key: people_big.country, people_big.department
         Batches: 1  Memory Usage: 1553kB
         Buffers: shared hit=9936
         ->  Gather  (cost=18227.67..20182.67 rows=18768 width=51) (actual time=428.359..431.326 rows=23460.00 loops=1)
               Workers Planned: 2
               Workers Launched: 2
               Buffers: shared hit=9936
               ->  Partial HashAggregate  (cost=17227.67..17305.87 rows=7820 width=51) (actual time=420.331..421.566 rows=7820.00 loops=3)
                     Group Key: people_big.country, people_big.department
                     Batches: 1  Memory Usage: 1553kB
                     Buffers: shared hit=9936
                     Worker 0:  Batches: 1  Memory Usage: 1553kB
                     Worker 1:  Batches: 1  Memory Usage: 1553kB
                     ->  Parallel Seq Scan on people_big  (cost=0.00..14102.67 rows=416667 width=23) (actual time=0.080..153.137 rows=333333.33 loops=3)
                           Buffers: shared hit=9936
 Planning Time: 0.559 ms
 Execution Time: 443.354 ms
```

