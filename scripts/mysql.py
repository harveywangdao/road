#!/usr/bin/python3

import pymysql

db = pymysql.connect("localhost","root","123456","tboxdb" )

cursor = db.cursor()

#create table
cursor.execute("DROP TABLE IF EXISTS EMPLOYEE")

sql = """CREATE TABLE EMPLOYEE (
         FIRST_NAME  CHAR(20) NOT NULL,
         LAST_NAME  CHAR(20),
         AGE INT,  
         SEX CHAR(1),
         INCOME FLOAT )"""

cursor.execute(sql)

#insert
sql = """INSERT INTO EMPLOYEE(FIRST_NAME,
         LAST_NAME, AGE, SEX, INCOME)
         VALUES ('Mac', 'Mohan', 20, 'M', 2000)"""

#sql = "INSERT INTO EMPLOYEE(FIRST_NAME, \
#       LAST_NAME, AGE, SEX, INCOME) \
#       VALUES ('%s', '%s', '%d', '%c', '%d' )" % \
#       ('Mac', 'Mohan', 20, 'M', 2000)

try:
    cursor.execute(sql)
    db.commit()
except:
    db.rollback()

#query
sql = "SELECT * FROM EMPLOYEE \
       WHERE INCOME > '%d'" % (1000)

try:
   cursor.execute(sql)

   results = cursor.fetchall()
   for row in results:
      fname = row[0]
      lname = row[1]
      age = row[2]
      sex = row[3]
      income = row[4]

      print ("fname=%s,lname=%s,age=%d,sex=%s,income=%d" % \
             (fname, lname, age, sex, income ))
except:
   print ("Error: unable to fetch data")

#update
sql = "UPDATE EMPLOYEE SET AGE = AGE + 1 WHERE SEX = '%c'" % ('M')
try:
   cursor.execute(sql)
   db.commit()
except:
   db.rollback()

#delete
sql = "DELETE FROM EMPLOYEE WHERE AGE > '%d'" % (20)
try:
   cursor.execute(sql)
   db.commit()
except:
   db.rollback()

db.close()