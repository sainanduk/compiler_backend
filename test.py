import requests
import json
import time

BASE_URL = "http://localhost:8001"

def test_endpoint(endpoint, code, language, input=""):
    url = f"{BASE_URL}/{endpoint}"
    data = {
        "language": language,
        "code": code,
        "input": input
    }
    
    print(f"\nTesting {endpoint} with {language}:")
    print(f"Code: {code}")
    
    try:
        response = requests.post(url, json=data)
        print(f"Status Code: {response.status_code}")
        if response.status_code == 200:
            print(f"Response: {json.dumps(response.json(), indent=2)}")
        else:
            print(f"Error Response: {response.text}")
        return response.status_code == 200
    except Exception as e:
        print(f"Error: {str(e)}")
        return False

def main():
    # Test Python
    python_code = "print('Hello from Python!')"
    test_endpoint("execute", python_code, "python")
    
    # Test Go
    go_code = """package main
import "fmt"
func main() {
    fmt.Println("Hello from Go!")
}"""
    test_endpoint("execute", go_code, "go")
    
    # Test C++
    cpp_code = """#include <iostream>
int main() {
    std::cout << "Hello from C++!" << std::endl;
    return 0;
}"""
    test_endpoint("execute", cpp_code, "cpp")
    
    # Test C
    c_code = """#include <stdio.h>
int main() {
    printf("Hello from C!\\n");
    return 0;
}"""
    test_endpoint("execute", c_code, "c")
    
    # Test submission endpoint
    python_submission = """def process_input(input_str):
    return f"Processed: {input_str}"

if __name__ == "__main__":
    user_input = input()
    result = process_input(user_input)
    print(result)"""
    test_endpoint("submit", python_submission, "python", "Test Input")

if __name__ == "__main__":
    main()
