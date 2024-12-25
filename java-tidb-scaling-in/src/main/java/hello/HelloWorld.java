package hello;

import org.joda.time.LocalTime;

public class HelloWorld {
    public static void main(String[] args) throws Exception  {
      LocalTime currentTime = new LocalTime();
		  System.out.println("The current local time is: " + currentTime);

        Greeter greeter = new Greeter();
        System.out.println(greeter.sayHello());

	try {
	    ReadDataFromDB obj = new ReadDataFromDB();
	    String query="SELECT * FROM `test01` limit 1";
	    obj.readDataFromDB(query);
        } catch (Exception e) {
            throw e;
        } finally {
	}
    }
}
