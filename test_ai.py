#!/usr/bin/env python3
"""Test script for AI processor"""

import json
import subprocess

def test_ai_processor():
    """Test the AI processor with a sample payload"""
    
    # Test payload (without real API key)
    test_payload = {
        "provider": "openai",
        "model": "gpt-4o-mini",
        "api_key": "test-key",
        "prompt": "Analyze this webpage content and extract key insights:",
        "content": "# Sample Website\n\nThis is a test webpage about artificial intelligence and machine learning. It contains information about various AI technologies and their applications in modern business."
    }
    
    try:
        # Test the Python script
        process = subprocess.Popen(
            ["python3", "ai_processor.py"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        
        stdout, stderr = process.communicate(input=json.dumps(test_payload))
        
        print("AI Processor Test Results:")
        print(f"Exit code: {process.returncode}")
        print(f"Output: {stdout}")
        if stderr:
            print(f"Errors: {stderr}")
            
    except Exception as e:
        print(f"Test failed: {e}")

if __name__ == "__main__":
    test_ai_processor()