SELECT *
from
    values
      (10,'a0'),
      (15,'a8'),
      (20,'a1'),
      (25,'a7'),
      (30,'a2'),
      (35,'a6'),
      (40,'a3'),
      (45,'a5'),
      (50,'a4'),
      (null,'a_null')
      l (a,sa)
left join
    values
      (20,'b20.1'),
      (20,'b20.2'),
      (40,'b40'),
      (40,'b40.2'),
      (60,'b60'),
      (null,'b_null')
      r (b,sb)
on l.a = r.b
order by a
